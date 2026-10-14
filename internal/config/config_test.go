package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
)

func sample() *Config {
	return &Config{Device: "web-1", Bucket: "my-hop", Region: "ap-southeast-1", AccessKeyID: "AKIA1", SecretAccessKey: "s3cr3t"}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	s := &Store{Path: filepath.Join(t.TempDir(), "nested", "config.json")}
	require.NoError(t, s.Save(sample()))

	info, err := os.Stat(s.Path)
	require.NoError(t, err)
	assert.Equal(t, File.FileMode, info.Mode().Perm())

	got, err := s.Load()
	require.NoError(t, err)
	assert.Equal(t, "web-1", got.Device)
	assert.Equal(t, CurrentVersion, got.Version)
}

func TestLoadRefusesReadableByOthers(t *testing.T) {
	s := &Store{Path: filepath.Join(t.TempDir(), "config.json")}
	require.NoError(t, s.Save(sample()))
	require.NoError(t, os.Chmod(s.Path, 0o644))

	_, err := s.Load()
	assert.ErrorContains(t, err, "chmod 600")
	assert.Equal(t, apperr.ExitAuth, apperr.CodeOf(err))
}

func TestLoadMissing(t *testing.T) {
	_, err := (&Store{Path: filepath.Join(t.TempDir(), "none.json")}).Load()
	assert.EqualError(t, err, "not configured — run 'hop init' or 'hop join <code>'")
}

func TestLoadCorruptAndVersion(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(bad, []byte("{"), 0o600))
	_, err := (&Store{Path: bad}).Load()
	assert.ErrorContains(t, err, "corrupt")

	future := filepath.Join(dir, "future.json")
	require.NoError(t, os.WriteFile(future, []byte(`{"version":9}`), 0o600))
	_, err = (&Store{Path: future}).Load()
	assert.ErrorContains(t, err, "unsupported version")
}

func TestDefaultStoreHonorsEnv(t *testing.T) {
	t.Setenv(File.EnvOverride, "/x/y.json")
	s, err := DefaultStore()
	require.NoError(t, err)
	assert.Equal(t, "/x/y.json", s.Path)
}

func TestStringHidesSecret(t *testing.T) {
	assert.NotContains(t, sample().String(), "s3cr3t")
}
