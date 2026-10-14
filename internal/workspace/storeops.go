package workspace

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/store"
	"github.com/ycniuqton/hop/internal/target"
)

// StoreManager owns the per-device store trees.
type StoreManager struct {
	ws        *Workspace
	devices   *DeviceManager
	transfers *Transfers
}

// Entry is one row of a store listing.
type Entry struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Dir      bool      `json:"dir"`
}

// Thresholds are the confirmation limits from spec section 5.
var Thresholds = struct{ ConfirmDelete int }{ConfirmDelete: 10}

func (m *StoreManager) resolveDevices(ctx context.Context, ts []target.Target) error {
	for _, dev := range target.List(ts).Devices() {
		if err := m.devices.Resolve(ctx, dev); err != nil {
			return err
		}
	}
	return nil
}

// classify reports whether a remote target is a folder (tree) or a single file, and its size.
func (m *StoreManager) classify(ctx context.Context, t target.Target) (bool, int64, error) {
	if t.Path == "" || t.Dir {
		return true, 0, nil
	}
	info, err := m.ws.Storage.Head(ctx, m.ws.Layout.StoreKey(t.Device, t.Path))
	if err == nil {
		return false, info.Size, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return false, 0, err
	}
	objs, _, err := m.ws.Storage.List(ctx, m.ws.Layout.DirPrefix(t.Device, t.Path), "", 1)
	if err != nil {
		return false, 0, err
	}
	if len(objs) == 0 {
		return false, 0, apperr.New(apperr.ExitNotFound, "no such path: %s", RemoteRef(t.Device, t.Path)).Wrap(store.ErrNotFound)
	}
	return true, 0, nil
}

// List shows one folder level (or everything below it with recursive): folders first, then files.
func (m *StoreManager) List(ctx context.Context, t target.Target, recursive bool) ([]Entry, error) {
	if err := m.devices.Resolve(ctx, t.Device); err != nil {
		return nil, err
	}
	isTree, _, err := m.classify(ctx, t)
	if err != nil {
		return nil, err
	}
	if !isTree {
		info, err := m.ws.Storage.Head(ctx, m.ws.Layout.StoreKey(t.Device, t.Path))
		return []Entry{{Name: t.Path, Size: info.Size, Modified: info.Modified}}, err
	}
	return m.listTree(ctx, t, recursive)
}

func (m *StoreManager) listTree(ctx context.Context, t target.Target, recursive bool) ([]Entry, error) {
	prefix := m.ws.Layout.DirPrefix(t.Device, t.Path)
	delim := Paths.Sep
	if recursive {
		delim = ""
	}
	objs, dirs, err := m.ws.Storage.List(ctx, prefix, delim, store.Limits.NoLimit)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(dirs)+len(objs))
	for _, d := range dirs {
		out = append(out, Entry{Name: strings.TrimPrefix(d, prefix), Dir: true})
	}
	for _, o := range objs {
		if name := strings.TrimPrefix(o.Key, prefix); name != "" && !strings.HasSuffix(name, Paths.Sep) {
			out = append(out, Entry{Name: name, Size: o.Size, Modified: o.Modified})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// Move is copy-then-delete between store paths (remote to remote only in v1).
func (m *StoreManager) Move(ctx context.Context, src, dst target.Target, force bool) (CopyResult, error) {
	if src.Kind != target.Remote || dst.Kind != target.Remote {
		return CopyResult{}, apperr.New(apperr.ExitUsage, "mv works between store paths only").
			WithHint("use @device:path on both sides, or 'hop cp' to upload or download")
	}
	if src.Device == dst.Device && src.Path == dst.Path {
		return CopyResult{}, apperr.New(apperr.ExitUsage, "source and destination are the same")
	}
	res, err := m.Copy(ctx, []target.Target{src}, dst, CopyOptions{Recursive: true, Force: force})
	if err != nil {
		return res, err
	}
	keys := make([]string, len(res.jobs))
	for i, j := range res.jobs {
		keys[i] = j.srcKey
	}
	return res, m.ws.Storage.Delete(ctx, keys...)
}

// ConfirmFunc asks the user whether to delete n objects.
type ConfirmFunc func(n int) (bool, error)

// Remove deletes store files, or whole folders with recursive.
func (m *StoreManager) Remove(ctx context.Context, ts []target.Target, recursive bool, confirm ConfirmFunc) (int, error) {
	if err := m.resolveDevices(ctx, ts); err != nil {
		return 0, err
	}
	var keys []string
	for _, t := range ts {
		more, err := m.keysToRemove(ctx, t, recursive)
		if err != nil {
			return 0, err
		}
		keys = append(keys, more...)
	}
	if recursive && len(keys) > Thresholds.ConfirmDelete {
		if ok, err := confirm(len(keys)); err != nil || !ok {
			return 0, err
		}
	}
	return len(keys), m.ws.Storage.Delete(ctx, keys...)
}

func (m *StoreManager) keysToRemove(ctx context.Context, t target.Target, recursive bool) ([]string, error) {
	if t.Kind != target.Remote {
		return nil, apperr.New(apperr.ExitUsage, "rm deletes store paths only: %s", t.Raw).WithHint("use @device:path")
	}
	isTree, _, err := m.classify(ctx, t)
	if err != nil {
		return nil, err
	}
	if !isTree {
		return []string{m.ws.Layout.StoreKey(t.Device, t.Path)}, nil
	}
	if !recursive {
		return nil, apperr.New(apperr.ExitUsage, "%s is a folder", t.Raw).WithHint("use -r")
	}
	objs, err := m.ws.listAll(ctx, m.ws.Layout.DirPrefix(t.Device, t.Path))
	keys := make([]string, len(objs))
	for i, o := range objs {
		keys[i] = o.Key
	}
	return keys, err
}
