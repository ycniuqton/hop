package cli

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/cache"
	"github.com/ycniuqton/hop/internal/config"
	"github.com/ycniuqton/hop/internal/store"
	"github.com/ycniuqton/hop/internal/target"
	"github.com/ycniuqton/hop/internal/ui"
	"github.com/ycniuqton/hop/internal/workspace"
)

// EnvVars are the environment variables hop reads.
var EnvVars = struct {
	Home, ConfigHome, CacheHome, Shell, NoColor, FPath string
	AWSKey, AWSSecret, AWSRegion, AWSDefaultRegion     string
}{
	Home: "HOME", ConfigHome: "XDG_CONFIG_HOME", CacheHome: "XDG_CACHE_HOME", Shell: "SHELL", NoColor: "NO_COLOR", FPath: "FPATH",
	AWSKey: "AWS_ACCESS_KEY_ID", AWSSecret: "AWS_SECRET_ACCESS_KEY", AWSRegion: "AWS_REGION", AWSDefaultRegion: "AWS_DEFAULT_REGION",
}

// Env is everything a command needs from the outside world; tests replace any part of it.
type Env struct {
	Opts     GlobalOptions
	Console  *ui.Console
	Getenv   func(string) string
	Hostname func() (string, error)
	Stores   *store.Manager
	Spawner  RefreshSpawner
	Version  string
	Now      func() time.Time

	root *cobra.Command
	cfg  *config.Config
	ws   *workspace.Workspace
	svc  *workspace.Services
}

// NewEnv builds the production environment on the real standard streams.
func NewEnv(version string) *Env {
	return &Env{
		Console:  ui.NewConsole(os.Stdin, os.Stdout, os.Stderr),
		Getenv:   os.Getenv,
		Hostname: os.Hostname,
		Stores:   store.NewManager(),
		Spawner:  processSpawner{},
		Version:  version,
		Now:      time.Now,
	}
}

func (e *Env) applyOptions() {
	e.Console.Quiet, e.Console.JSON = e.Opts.Quiet, e.Opts.JSON
	if e.Opts.NoColor || e.Getenv(EnvVars.NoColor) != "" {
		e.Console.Color = false
	}
}

func (e *Env) home() string {
	if h := e.Getenv(EnvVars.Home); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

// ConfigStore resolves HOP_CONFIG, then $XDG_CONFIG_HOME, then ~/.config.
func (e *Env) ConfigStore() *config.Store {
	if p := e.Getenv(config.File.EnvOverride); p != "" {
		return &config.Store{Path: p}
	}
	dir := e.Getenv(EnvVars.ConfigHome)
	if dir == "" {
		dir = filepath.Join(e.home(), ".config")
	}
	return &config.Store{Path: filepath.Join(dir, config.File.AppDir, config.File.FileName)}
}

// CacheRoot is $XDG_CACHE_HOME or ~/.cache.
func (e *Env) CacheRoot() string {
	if dir := e.Getenv(EnvVars.CacheHome); dir != "" {
		return dir
	}
	return filepath.Join(e.home(), ".cache")
}

// Config loads the local config once per run.
func (e *Env) Config() (*config.Config, error) {
	if e.cfg != nil {
		return e.cfg, nil
	}
	cfg, err := e.ConfigStore().Load()
	if err != nil {
		return nil, err
	}
	e.cfg = cfg
	return cfg, nil
}

// Settings converts a config into storage connection settings.
func (e *Env) Settings(cfg *config.Config) store.Settings {
	s := store.Settings{Bucket: cfg.Bucket, Region: cfg.Region, Endpoint: cfg.Endpoint,
		AccessKeyID: cfg.AccessKeyID, SecretAccessKey: cfg.SecretAccessKey}
	if e.Opts.Verbose {
		s.Trace = e.Console.Err
	}
	return s
}

// OpenWorkspace connects to the bucket described by cfg; it does not require a registered device.
func (e *Env) OpenWorkspace(ctx context.Context, cfg *config.Config) (*workspace.Workspace, error) {
	s, err := e.Stores.Open(ctx, e.Settings(cfg))
	if err != nil {
		return nil, err
	}
	ws := &workspace.Workspace{
		Storage: s, Layout: workspace.NewLayout(cfg.Prefix), Self: cfg.Device, Now: e.Now, Version: e.Version,
		Meter: func(label string, total int64) workspace.Meter { return e.Console.Progress(label, total) },
	}
	if cfg.WorkspaceID != "" {
		ws.Cache = cache.Open(e.CacheRoot(), cfg.WorkspaceID)
		ws.Cache.Now = e.Now
	}
	return ws, nil
}

// Services opens the configured workspace and its managers.
func (e *Env) Services(ctx context.Context) (*workspace.Services, error) {
	if e.svc != nil {
		return e.svc, nil
	}
	cfg, err := e.Config()
	if err != nil {
		return nil, err
	}
	ws, err := e.OpenWorkspace(ctx, cfg)
	if err != nil {
		return nil, err
	}
	e.ws, e.svc = ws, workspace.NewServices(ws)
	return e.svc, nil
}

// Parser resolves @ against the configured device.
func (e *Env) Parser() (target.Parser, error) {
	cfg, err := e.Config()
	if err != nil {
		return target.Parser{}, err
	}
	return target.Parser{Self: cfg.Device}, nil
}

// touch records last_seen after a successful command that used the workspace.
func (e *Env) touch(ctx context.Context) {
	if e.svc != nil {
		_ = e.svc.Devices.Touch(ctx)
	}
}
