package workspace

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/cache"
	"github.com/ycniuqton/hop/internal/naming"
	"github.com/ycniuqton/hop/internal/store"
	"github.com/ycniuqton/hop/internal/suggest"
	"github.com/ycniuqton/hop/internal/target"
)

// HotFile is one entry on a hot shelf.
type HotFile struct {
	Shelf    Shelf     `json:"shelf"`
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	From     string    `json:"from,omitempty"`
}

// ShelfManager owns the shared and per-device hot shelves.
type ShelfManager struct {
	ws        *Workspace
	devices   *DeviceManager
	transfers *Transfers
}

// Resolve picks the shelf for an optional @device target: none → shared, @ → self.
func (m *ShelfManager) Resolve(ctx context.Context, t *target.Target) (Shelf, error) {
	if t == nil {
		return SharedShelf, nil
	}
	if err := m.devices.Resolve(ctx, t.Device); err != nil {
		return "", err
	}
	return Shelf(t.Device), nil
}

// Push uploads one local file under its base name or as.
func (m *ShelfManager) Push(ctx context.Context, shelf Shelf, local, as string) (HotFile, error) {
	name := m.nameFor(filepath.Base(local), as)
	if err := naming.HotNames.Validate(name); err != nil {
		return HotFile{}, err
	}
	f, err := m.transfers.OpenLocal(local)
	if err != nil {
		return HotFile{}, err
	}
	if f.Mode.IsDir() {
		return HotFile{}, apperr.New(apperr.ExitUsage, "push sends single files").WithHint("use 'hop cp -r' for folders")
	}
	if err := m.transfers.Upload(ctx, f, m.ws.Layout.HotKey(string(shelf), name)); err != nil {
		return HotFile{}, err
	}
	return m.recordPushed(shelf, name, f.Size), nil
}

// PushFromStore copies a store file onto a shelf server-side.
func (m *ShelfManager) PushFromStore(ctx context.Context, shelf Shelf, src target.Target, as string) (HotFile, error) {
	key := m.ws.Layout.StoreKey(src.Device, src.Path)
	info, err := m.ws.Storage.Head(ctx, key)
	if errors.Is(err, store.ErrNotFound) || (err == nil && src.Path == "") {
		return HotFile{}, apperr.New(apperr.ExitNotFound, "no such store file: %s", RemoteRef(src.Device, src.Path))
	}
	if err != nil {
		return HotFile{}, err
	}
	name := m.nameFor(path.Base(src.Path), as)
	if err := naming.HotNames.Validate(name); err != nil {
		return HotFile{}, err
	}
	if err := m.ws.Storage.Copy(ctx, key, m.ws.Layout.HotKey(string(shelf), name)); err != nil {
		return HotFile{}, err
	}
	return m.recordPushed(shelf, name, info.Size), nil
}

func (m *ShelfManager) nameFor(base, as string) string {
	if as != "" {
		return as
	}
	return base
}

func (m *ShelfManager) recordPushed(shelf Shelf, name string, size int64) HotFile {
	hf := HotFile{Shelf: shelf, Name: name, Size: size, Modified: m.ws.Now().UTC(), From: m.ws.Self}
	if m.ws.Cache != nil {
		m.ws.Cache.UpsertShelf(string(shelf), cache.ShelfItem{Name: name, Size: size, Modified: hf.Modified})
	}
	return hf
}

// List returns a shelf's files newest first, refreshing the cache; withFrom adds the sender.
func (m *ShelfManager) List(ctx context.Context, shelf Shelf, withFrom bool) ([]HotFile, error) {
	prefix := m.ws.Layout.ShelfPrefix(string(shelf))
	objs, err := m.ws.listAll(ctx, prefix)
	if err != nil {
		return nil, err
	}
	var files []HotFile
	for _, o := range objs {
		name := strings.TrimPrefix(o.Key, prefix)
		if name != "" && !strings.Contains(name, Paths.Sep) {
			files = append(files, HotFile{Shelf: shelf, Name: name, Size: o.Size, Modified: o.Modified})
		}
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].Modified.After(files[j].Modified) })
	m.saveCache(shelf, files)
	if !withFrom {
		return files, nil
	}
	return files, m.fillFrom(ctx, files)
}

func (m *ShelfManager) saveCache(shelf Shelf, files []HotFile) {
	if m.ws.Cache == nil {
		return
	}
	items := make([]cache.ShelfItem, len(files))
	for i, f := range files {
		items[i] = cache.ShelfItem{Name: f.Name, Size: f.Size, Modified: f.Modified}
	}
	m.ws.Cache.SaveShelf(string(shelf), items)
}

func (m *ShelfManager) fillFrom(ctx context.Context, files []HotFile) error {
	return Pool{Workers: Concurrency.Lookups}.Run(ctx, len(files), func(ctx context.Context, i int) error {
		info, err := m.ws.Storage.Head(ctx, m.ws.Layout.HotKey(string(files[i].Shelf), files[i].Name))
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		files[i].From = info.Meta[Meta.From]
		return err
	})
}

// Shelves lists every shelf that currently holds files.
func (m *ShelfManager) Shelves(ctx context.Context) ([]Shelf, error) {
	_, dirs, err := m.ws.Storage.List(ctx, m.ws.Layout.HotRoot(), Paths.Sep, store.Limits.NoLimit)
	if err != nil {
		return nil, err
	}
	out := make([]Shelf, 0, len(dirs))
	for _, d := range dirs {
		out = append(out, Shelf(strings.TrimSuffix(strings.TrimPrefix(d, m.ws.Layout.HotRoot()), Paths.Sep)))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].IsShared() && !out[j].IsShared() })
	return out, nil
}

// Locate finds a hot file or explains what the user probably meant.
func (m *ShelfManager) Locate(ctx context.Context, shelf Shelf, name string) (store.Object, error) {
	if err := naming.HotNames.Validate(name); err != nil {
		return store.Object{}, err
	}
	info, err := m.ws.Storage.Head(ctx, m.ws.Layout.HotKey(string(shelf), name))
	if errors.Is(err, store.ErrNotFound) {
		return info, m.missing(ctx, shelf, name)
	}
	return info, err
}

func (m *ShelfManager) missing(ctx context.Context, shelf Shelf, name string) error {
	e := apperr.New(apperr.ExitNotFound, "%q not on %s", name, shelf.Label()).Wrap(store.ErrNotFound)
	if shelf.IsShared() && m.ws.Self != "" {
		if _, err := m.ws.Storage.Head(ctx, m.ws.Layout.HotKey(m.ws.Self, name)); err == nil {
			return e.WithHint("did you mean: hop pull %s @", name)
		}
	}
	files, err := m.List(ctx, shelf, false)
	if err != nil {
		return e
	}
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	if s := suggest.DidYouMean.Closest(name, names); s != "" {
		return e.WithHint("did you mean %s?", s)
	}
	return e
}

// Pull downloads a hot file; with remove it deletes the object after the file is in place.
func (m *ShelfManager) Pull(ctx context.Context, shelf Shelf, name, dest string, force, remove bool) (string, error) {
	if _, err := m.Locate(ctx, shelf, name); err != nil {
		return "", err
	}
	dest = m.destFor(name, dest)
	key := m.ws.Layout.HotKey(string(shelf), name)
	if _, err := m.transfers.Download(ctx, key, dest, force); err != nil {
		return "", err
	}
	if !remove {
		return dest, nil
	}
	if err := m.ws.Storage.Delete(ctx, key); err != nil {
		return dest, err
	}
	if m.ws.Cache != nil {
		m.ws.Cache.RemoveFromShelf(string(shelf), name)
	}
	return dest, nil
}

func (m *ShelfManager) destFor(name, dest string) string {
	if dest == "" {
		return name
	}
	if info, err := os.Stat(dest); (err == nil && info.IsDir()) || strings.HasSuffix(dest, string(filepath.Separator)) {
		return filepath.Join(dest, name)
	}
	return dest
}

// Remove deletes named files from a shelf.
func (m *ShelfManager) Remove(ctx context.Context, shelf Shelf, names []string) error {
	keys := make([]string, len(names))
	for i, n := range names {
		if _, err := m.Locate(ctx, shelf, n); err != nil {
			return err
		}
		keys[i] = m.ws.Layout.HotKey(string(shelf), n)
	}
	if err := m.ws.Storage.Delete(ctx, keys...); err != nil {
		return err
	}
	if m.ws.Cache != nil {
		m.ws.Cache.RemoveFromShelf(string(shelf), names...)
	}
	return nil
}

// Clear deletes every file on a shelf and returns how many were removed.
func (m *ShelfManager) Clear(ctx context.Context, shelf Shelf) (int, error) {
	files, err := m.List(ctx, shelf, false)
	if err != nil {
		return 0, err
	}
	keys := make([]string, len(files))
	for i, f := range files {
		keys[i] = m.ws.Layout.HotKey(string(shelf), f.Name)
	}
	if err := m.ws.Storage.Delete(ctx, keys...); err != nil {
		return 0, err
	}
	m.saveCache(shelf, nil)
	return len(keys), nil
}

// Keep copies a hot file into the store server-side (hot → store bridge).
func (m *ShelfManager) Keep(ctx context.Context, shelf Shelf, name string, dst target.Target, force bool) (string, error) {
	if _, err := m.Locate(ctx, shelf, name); err != nil {
		return "", err
	}
	if err := m.devices.Resolve(ctx, dst.Device); err != nil {
		return "", err
	}
	p := dst.Path
	if dst.Dir {
		p = path.Join(dst.Path, name)
	}
	key := m.ws.Layout.StoreKey(dst.Device, p)
	if _, err := m.ws.Storage.Head(ctx, key); err == nil && !force {
		return "", apperr.New(apperr.ExitConflict, "exists: %s (use -f to overwrite)", RemoteRef(dst.Device, p))
	}
	return RemoteRef(dst.Device, p), m.ws.Storage.Copy(ctx, m.ws.Layout.HotKey(string(shelf), name), key)
}
