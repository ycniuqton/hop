// Package config loads and saves the per-device local config (~/.config/hop/config.json).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ycniuqton/hop/internal/apperr"
)

// Config is this device's identity and S3 credentials.
type Config struct {
	Version         int    `json:"version"`
	Device          string `json:"device"`
	Bucket          string `json:"bucket"`
	Region          string `json:"region"`
	Prefix          string `json:"prefix"`
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	WorkspaceID     string `json:"workspace_id"`
}

// CurrentVersion is the config schema version written by this build.
const CurrentVersion = 1

// String never reveals the secret key.
func (c Config) String() string {
	return fmt.Sprintf("device=%s bucket=%s region=%s prefix=%q endpoint=%q key=%s secret=***",
		c.Device, c.Bucket, c.Region, c.Prefix, c.Endpoint, c.AccessKeyID)
}

// FileSpec describes where and how the config file is stored.
type FileSpec struct {
	EnvOverride string
	AppDir      string
	FileName    string
	FileMode    fs.FileMode
	DirMode     fs.FileMode
	ForbidMask  fs.FileMode
}

// File is the config file policy from spec section 3.
var File = FileSpec{
	EnvOverride: "HOP_CONFIG",
	AppDir:      "hop",
	FileName:    "config.json",
	FileMode:    0o600,
	DirMode:     0o700,
	ForbidMask:  0o077,
}

// Store reads and writes one config file.
type Store struct {
	Path string
}

// DefaultStore resolves the config path from HOP_CONFIG or the user config directory.
func DefaultStore() (*Store, error) {
	if p := os.Getenv(File.EnvOverride); p != "" {
		return &Store{Path: p}, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, apperr.New(apperr.ExitGeneral, "cannot locate config directory: %v", err)
	}
	return &Store{Path: filepath.Join(dir, File.AppDir, File.FileName)}, nil
}

// Exists reports whether a config file is present.
func (s *Store) Exists() bool {
	_, err := os.Stat(s.Path)
	return err == nil
}

// Load reads the config, refusing files readable by group or others.
func (s *Store) Load() (*Config, error) {
	info, err := os.Stat(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, apperr.New(apperr.ExitGeneral, "not configured").WithHint("run 'hop init' or 'hop join <code>'")
	}
	if err != nil {
		return nil, apperr.New(apperr.ExitGeneral, "cannot read config %s: %v", s.Path, err)
	}
	if info.Mode().Perm()&File.ForbidMask != 0 {
		return nil, apperr.New(apperr.ExitAuth, "config %s is readable by other users (mode %04o)", s.Path, info.Mode().Perm()).
			WithHint("run 'chmod 600 %s'", s.Path)
	}
	return s.decode()
}

func (s *Store) decode() (*Config, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, apperr.New(apperr.ExitGeneral, "cannot read config %s: %v", s.Path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, apperr.New(apperr.ExitGeneral, "config %s is corrupt: %v", s.Path, err)
	}
	if c.Version != CurrentVersion {
		return nil, apperr.New(apperr.ExitGeneral, "config %s has unsupported version %d", s.Path, c.Version).
			WithHint("upgrade hop")
	}
	return &c, nil
}

// Save writes the config atomically with mode 0600.
func (s *Store) Save(c *Config) error {
	c.Version = CurrentVersion
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, File.DirMode); err != nil {
		return apperr.New(apperr.ExitGeneral, "cannot create %s: %v", dir, err)
	}
	return s.writeAtomic(append(data, '\n'))
}

func (s *Store) writeAtomic(data []byte) error {
	path, mode := s.Path, File.FileMode
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return apperr.New(apperr.ExitGeneral, "cannot write %s: %v", path, err)
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return apperr.New(apperr.ExitGeneral, "cannot write %s: %v", path, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
