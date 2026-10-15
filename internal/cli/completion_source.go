package cli

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ycniuqton/hop/internal/cache"
	"github.com/ycniuqton/hop/internal/complete"
	"github.com/ycniuqton/hop/internal/config"
	"github.com/ycniuqton/hop/internal/store"
	"github.com/ycniuqton/hop/internal/workspace"
)

// CompletionLimits caps the live store listing used by completion (spec section 7).
var CompletionLimits = struct {
	Timeout time.Duration
	MaxKeys int
}{Timeout: time.Second, MaxKeys: 1000}

// RefreshTarget names what a background refresh updates.
type RefreshTarget string

const (
	RefreshDevices RefreshTarget = "devices"
	RefreshShelf   RefreshTarget = "shelf"
)

// RefreshSpawner starts `hop __refresh` without waiting for it.
type RefreshSpawner interface {
	Spawn(what RefreshTarget, args ...string)
}

type processSpawner struct{}

func (processSpawner) Spawn(what RefreshTarget, args ...string) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, append([]string{refreshCommandName, string(what)}, args...)...)
	processSpawner{}.detach(cmd)
	if cmd.Start() == nil {
		_ = cmd.Process.Release()
	}
}

func (e *Env) completer() complete.Completer {
	return complete.Completer{Src: &completionSource{env: e}}
}

// completionSource answers from the local cache and spawns refreshes for stale entries.
type completionSource struct {
	env    *Env
	cfg    *config.Config
	loaded bool
}

func (s *completionSource) config() *config.Config {
	if !s.loaded {
		s.loaded = true
		s.cfg, _ = s.env.ConfigStore().Load()
	}
	return s.cfg
}

func (s *completionSource) cache() *cache.Cache {
	cfg := s.config()
	if cfg == nil || cfg.WorkspaceID == "" {
		return nil
	}
	c := cache.Open(s.env.CacheRoot(), cfg.WorkspaceID)
	c.Now = s.env.Now
	return c
}

func (s *completionSource) Self() string {
	if cfg := s.config(); cfg != nil {
		return cfg.Device
	}
	return ""
}

func (s *completionSource) Devices() []string {
	c := s.cache()
	if c == nil {
		return nil
	}
	snap, ok := c.Devices()
	if !ok || c.Stale(snap.FetchedAt) {
		s.env.Spawner.Spawn(RefreshDevices)
	}
	out := make([]string, len(snap.Items))
	for i, d := range snap.Items {
		out[i] = d.Name
	}
	return out
}

func (s *completionSource) ShelfNames(shelf string) []string {
	c := s.cache()
	if c == nil {
		return nil
	}
	snap, ok := c.Shelf(shelf)
	if !ok || c.Stale(snap.FetchedAt) {
		s.env.Spawner.Spawn(RefreshShelf, shelf)
	}
	out := make([]string, len(snap.Items))
	for i, it := range snap.Items {
		out[i] = it.Name
	}
	return out
}

func (s *completionSource) StoreChildren(device, dir string) ([]string, []string) {
	cfg := s.config()
	if cfg == nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), CompletionLimits.Timeout)
	defer cancel()
	st, err := s.env.Stores.Open(ctx, s.env.Settings(cfg))
	if err != nil {
		return nil, nil
	}
	prefix := workspace.NewLayout(cfg.Prefix).StorePrefix(device) + dir
	objs, dirs, err := st.List(ctx, prefix, workspace.Paths.Sep, CompletionLimits.MaxKeys)
	if err != nil {
		return nil, nil
	}
	return s.relative(prefix, dirs, objs)
}

func (s *completionSource) relative(prefix string, dirs []string, objs []store.Object) ([]string, []string) {
	outDirs := make([]string, 0, len(dirs))
	for _, d := range dirs {
		outDirs = append(outDirs, strings.TrimPrefix(d, prefix))
	}
	files := make([]string, 0, len(objs))
	for _, o := range objs {
		if name := strings.TrimPrefix(o.Key, prefix); name != "" {
			files = append(files, name)
		}
	}
	return outDirs, files
}
