package selfinstall_test

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/selfinstall"
	"github.com/ycniuqton/hop/internal/selfinstall/selfinstalltest"
)

func TestVersionOrder(t *testing.T) {
	ordered := []string{"dev", "0.2.0-dev", "v0.2.0", "0.2.1", "0.10.0", "1.0.0"}
	for i := range ordered {
		for j := range ordered {
			want := map[bool]int{true: -1, false: 1}[i < j]
			if i == j {
				want = 0
			}
			got := selfinstall.ParseVersion(ordered[i]).Compare(selfinstall.ParseVersion(ordered[j]))
			assert.Equal(t, want, got, "%s vs %s", ordered[i], ordered[j])
		}
	}
	assert.False(t, selfinstall.ParseVersion("1.2").Valid)
	assert.False(t, selfinstall.ParseVersion("1.2.3x").Valid)
}

func TestLatestAndFetch(t *testing.T) {
	srv := selfinstalltest.Start(t, "me/hop", "v9.9.9", []byte("new-binary"))
	rel := selfinstall.NewReleases(srv.URL, "me/hop")
	tag, err := rel.Latest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "v9.9.9", tag)
	data, err := rel.Fetch(context.Background(), tag, runtime.GOOS, runtime.GOARCH)
	require.NoError(t, err)
	assert.Equal(t, "new-binary", string(data))

	_, err = rel.Fetch(context.Background(), tag, "plan9", "mips")
	assert.Equal(t, apperr.ExitNotFound, apperr.CodeOf(err))
	srv.BadSum = true
	_, err = rel.Fetch(context.Background(), tag, runtime.GOOS, runtime.GOARCH)
	assert.ErrorContains(t, err, "checksum mismatch")

	_, err = selfinstall.NewReleases(srv.URL, "nobody/none").Latest(context.Background())
	assert.Equal(t, apperr.ExitNotFound, apperr.CodeOf(err))
}

func TestReplaceFollowsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need admin rights on Windows")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "hop-real")
	require.NoError(t, os.WriteFile(real, []byte("old"), 0o755))
	link := filepath.Join(dir, "hop")
	require.NoError(t, os.Symlink(real, link))
	bin, err := selfinstall.Locate(func() (string, error) { return link, nil })
	require.NoError(t, err)
	require.NoError(t, bin.Replace([]byte("new")))
	got, _ := os.ReadFile(real)
	assert.Equal(t, "new", string(got))
	info, _ := os.Lstat(link)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "the symlink stays a symlink")
	st, _ := os.Stat(real)
	assert.Equal(t, os.FileMode(0o755), st.Mode().Perm())
	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 2, "no temp files left")
}

func TestReplaceWithoutWriteAccess(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs Unix directory permissions and a non-root user")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "hop")
	require.NoError(t, os.WriteFile(p, []byte("old"), 0o755))
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	err := selfinstall.Binary{Path: p}.Replace([]byte("new"))
	assert.ErrorContains(t, err, "re-run with sudo")
}

func TestFootprintRemove(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config", "hop", "config.json")
	cache := filepath.Join(dir, "cache", "hop")
	require.NoError(t, os.MkdirAll(filepath.Dir(cfg), 0o700))
	require.NoError(t, os.WriteFile(cfg, []byte("{}"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(cache, "ws"), 0o755))
	fp := selfinstall.Footprint{Items: []selfinstall.Item{
		{Kind: selfinstall.KindConfig, Path: cfg}, {Kind: selfinstall.KindCache, Path: cache, Dir: true},
		{Kind: selfinstall.KindSkill, Path: filepath.Join(dir, "missing")},
	}}
	items := fp.Existing()
	require.Len(t, items, 2)
	removed, err := fp.Remove(items)
	require.NoError(t, err)
	assert.Len(t, removed, 2)
	for _, p := range []string{cfg, filepath.Dir(cfg), cache} {
		_, err := os.Stat(p)
		assert.True(t, os.IsNotExist(err), p)
	}
}

func TestPackagingPerOS(t *testing.T) {
	rel := selfinstall.NewReleases("", "")
	assert.Equal(t, "hop_windows_arm64.zip", rel.Asset("windows", "arm64"))
	assert.Equal(t, "hop_linux_amd64.tar.gz", rel.Asset("linux", "amd64"))
	assert.Equal(t, "hop.exe", selfinstall.PackagingFor("windows").Executable)
	assert.Equal(t, "hop", selfinstall.PackagingFor("darwin").Executable)
}

func TestZipExtract(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{"README.md": "r", "hop.exe": "windows-binary"} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	require.NoError(t, zw.Close())
	got, err := selfinstall.PackagingFor("windows").Format.Extract(buf.Bytes(), "hop.exe")
	require.NoError(t, err)
	assert.Equal(t, "windows-binary", string(got))
	_, err = selfinstall.PackagingFor("windows").Format.Extract(buf.Bytes(), "missing.exe")
	assert.ErrorContains(t, err, `no "missing.exe"`)
	_, err = selfinstall.PackagingFor("windows").Format.Extract([]byte("not a zip"), "hop.exe")
	assert.ErrorContains(t, err, "not a zip")
}

func TestReplaceSwapsAndCleansStale(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "hop")
	require.NoError(t, os.WriteFile(p, []byte("old"), 0o755))
	bin := selfinstall.Binary{Path: p}
	require.NoError(t, bin.Replace([]byte("new")))
	got, _ := os.ReadFile(p)
	assert.Equal(t, "new", string(got))
	require.NoError(t, os.WriteFile(p+selfinstall.StaleSuffix, []byte("left over"), 0o644))
	bin.CleanStale()
	assert.NoFileExists(t, p+selfinstall.StaleSuffix)
	bin.CleanStale()
}
