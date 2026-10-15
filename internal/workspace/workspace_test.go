package workspace

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/cache"
	"github.com/ycniuqton/hop/internal/store"
	"github.com/ycniuqton/hop/internal/target"
)

var ctx = context.Background()

type device struct {
	ws  *Workspace
	svc *Services
	p   target.Parser
}

func newDevice(t *testing.T, s store.Storage, name string) device {
	ws := &Workspace{Storage: s, Layout: NewLayout("team"), Now: time.Now, Version: "test",
		Cache: cache.Open(t.TempDir(), "ws")}
	setup := NewSetup(ws)
	_, err := setup.Register(ctx, name, name+".local")
	require.NoError(t, err)
	return device{ws: ws, svc: NewServices(ws), p: target.Parser{Self: name}}
}

func twoDevices(t *testing.T) (device, device, *store.Fake) {
	fake := store.NewFake()
	setup := NewSetup(&Workspace{Storage: fake, Layout: NewLayout("team"), Now: time.Now})
	_, err := setup.Create(ctx, "laptop")
	require.NoError(t, err)
	return newDevice(t, fake, "laptop"), newDevice(t, fake, "web-1"), fake
}

func (d device) target(t *testing.T, arg string) target.Target {
	tg, err := d.p.Parse(arg)
	require.NoError(t, err)
	return tg
}

func writeFile(t *testing.T, dir, name, body string) string {
	p := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(body), 0o640))
	return p
}

func TestCreateTwiceFails(t *testing.T) {
	fake := store.NewFake()
	setup := NewSetup(&Workspace{Storage: fake, Layout: NewLayout(""), Now: time.Now})
	info, err := setup.Create(ctx, "laptop")
	require.NoError(t, err)
	assert.Len(t, info.WorkspaceID, 36)
	_, err = setup.Create(ctx, "laptop")
	assert.ErrorContains(t, err, "bucket already has a hop workspace")
	assert.Equal(t, apperr.ExitConflict, apperr.CodeOf(err))
}

func TestRegisterNameCollision(t *testing.T) {
	_, web, fake := twoDevices(t)
	_, err := NewSetup(&Workspace{Storage: fake, Layout: web.ws.Layout, Now: time.Now}).Register(ctx, "web-1", "other")
	assert.ErrorIs(t, err, ErrNameTaken)
	assert.ErrorIs(t, err, store.ErrPreconditionFailed)
	assert.EqualError(t, err, `device name "web-1" is already used — pick another with --name`)
}

func TestVerifyNamesFailingStep(t *testing.T) {
	fake := store.NewFake()
	fake.Missing = true
	err := NewSetup(&Workspace{Storage: fake, Layout: NewLayout(""), Now: time.Now}).Verify(ctx)
	assert.ErrorContains(t, err, "credential check failed at HeadBucket")
	fake.Missing = false
	assert.NoError(t, NewSetup(&Workspace{Storage: fake, Layout: NewLayout(""), Now: time.Now}).Verify(ctx))
	objs, _, _ := fake.List(ctx, "", "", 0)
	assert.Empty(t, objs, "probe object must be cleaned up")
}

func TestPushPullSharedAndDeviceShelves(t *testing.T) {
	laptop, web, _ := twoDevices(t)
	dir := t.TempDir()
	src := writeFile(t, dir, "dump.sql", "v1")

	_, err := laptop.svc.Shelves.Push(ctx, SharedShelf, src, "")
	require.NoError(t, err)
	_, err = laptop.svc.Shelves.Push(ctx, Shelf("web-1"), src, "")
	require.NoError(t, err)
	writeFile(t, dir, "dump.sql", "v2")
	_, err = laptop.svc.Shelves.Push(ctx, SharedShelf, src, "")
	require.NoError(t, err, "same name overwrites silently")

	out := t.TempDir()
	got, err := web.svc.Shelves.Pull(ctx, SharedShelf, "dump.sql", out, false, false)
	require.NoError(t, err)
	data, _ := os.ReadFile(got)
	assert.Equal(t, "v2", string(data))
	info, _ := os.Stat(got)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o640), info.Mode().Perm(), "mode restored")
	}

	_, err = web.svc.Shelves.Pull(ctx, SharedShelf, "dump.sql", out, false, false)
	assert.ErrorContains(t, err, "exists:")
	_, err = web.svc.Shelves.Pull(ctx, Shelf("web-1"), "dump.sql", filepath.Join(out, "mine.sql"), false, false)
	require.NoError(t, err)

	files, err := web.svc.Shelves.List(ctx, SharedShelf, true)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "laptop", files[0].From)
}

func TestPullRemoveAndHints(t *testing.T) {
	laptop, web, _ := twoDevices(t)
	src := writeFile(t, t.TempDir(), "dump.sql", "x")
	_, err := laptop.svc.Shelves.Push(ctx, Shelf("web-1"), src, "")
	require.NoError(t, err)

	_, err = web.svc.Shelves.Pull(ctx, SharedShelf, "dump.sql", t.TempDir(), false, false)
	assert.EqualError(t, err, `"dump.sql" not on shared shelf — did you mean: hop pull dump.sql @`)

	_, err = web.svc.Shelves.Pull(ctx, Shelf("web-1"), "dmp.sql", t.TempDir(), false, false)
	assert.EqualError(t, err, `"dmp.sql" not on @web-1's shelf — did you mean dump.sql?`)

	_, err = web.svc.Shelves.Pull(ctx, Shelf("web-1"), "dump.sql", t.TempDir(), false, true)
	require.NoError(t, err)
	files, _ := web.svc.Shelves.List(ctx, Shelf("web-1"), false)
	assert.Empty(t, files, "--rm deletes after download")
}

func TestPushRejectsDirectoryAndPathNames(t *testing.T) {
	laptop, _, _ := twoDevices(t)
	_, err := laptop.svc.Shelves.Push(ctx, SharedShelf, t.TempDir(), "")
	assert.EqualError(t, err, "push sends single files — use 'hop cp -r' for folders")
	_, err = laptop.svc.Shelves.Push(ctx, SharedShelf, writeFile(t, t.TempDir(), "a", "x"), "x/y")
	assert.ErrorContains(t, err, "hot names are flat")
}

func TestUnknownDeviceSuggestion(t *testing.T) {
	laptop, _, _ := twoDevices(t)
	tg := laptop.target(t, "@web1")
	_, err := laptop.svc.Shelves.Resolve(ctx, &tg)
	assert.EqualError(t, err, `unknown device "web1" — did you mean web-1?`)
	assert.Equal(t, apperr.ExitNotFound, apperr.CodeOf(err))
}

func TestClearAndRemove(t *testing.T) {
	laptop, _, _ := twoDevices(t)
	dir := t.TempDir()
	for _, n := range []string{"a", "b", "c"} {
		_, err := laptop.svc.Shelves.Push(ctx, SharedShelf, writeFile(t, dir, n, n), "")
		require.NoError(t, err)
	}
	require.NoError(t, laptop.svc.Shelves.Remove(ctx, SharedShelf, []string{"a"}))
	assert.Error(t, laptop.svc.Shelves.Remove(ctx, SharedShelf, []string{"zzz"}))
	n, err := laptop.svc.Shelves.Clear(ctx, SharedShelf)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}

func TestHotCacheWriteThrough(t *testing.T) {
	laptop, _, _ := twoDevices(t)
	_, err := laptop.svc.Shelves.Push(ctx, SharedShelf, writeFile(t, t.TempDir(), "a.txt", "1"), "")
	require.NoError(t, err)
	snap, ok := laptop.ws.Cache.Shelf(string(SharedShelf))
	require.True(t, ok)
	assert.Equal(t, "a.txt", snap.Items[0].Name)
	require.NoError(t, laptop.svc.Shelves.Remove(ctx, SharedShelf, []string{"a.txt"}))
	snap, _ = laptop.ws.Cache.Shelf(string(SharedShelf))
	assert.Empty(t, snap.Items)
}

func TestKeepAndPushFromStore(t *testing.T) {
	laptop, _, _ := twoDevices(t)
	_, err := laptop.svc.Shelves.Push(ctx, SharedShelf, writeFile(t, t.TempDir(), "db.sql", "x"), "")
	require.NoError(t, err)
	ref, err := laptop.svc.Shelves.Keep(ctx, SharedShelf, "db.sql", laptop.target(t, "@web-1:backups/"), false)
	require.NoError(t, err)
	assert.Equal(t, "@web-1:backups/db.sql", ref)
	_, err = laptop.svc.Shelves.Keep(ctx, SharedShelf, "db.sql", laptop.target(t, "@web-1:backups/"), false)
	assert.ErrorContains(t, err, "exists: @web-1:backups/db.sql (use -f to overwrite)")

	hf, err := laptop.svc.Shelves.PushFromStore(ctx, Shelf("laptop"), laptop.target(t, "@web-1:backups/db.sql"), "restore.sql")
	require.NoError(t, err)
	assert.Equal(t, "restore.sql", hf.Name)
}

func TestStatusUsage(t *testing.T) {
	laptop, _, _ := twoDevices(t)
	_, err := laptop.svc.Shelves.Push(ctx, SharedShelf, writeFile(t, t.TempDir(), "a", "12345"), "")
	require.NoError(t, err)
	_, err = laptop.svc.Store.Copy(ctx, []target.Target{{Kind: target.Local, Path: writeFile(t, t.TempDir(), "b", "12")}},
		laptop.target(t, "@web-1:"), CopyOptions{})
	require.NoError(t, err)
	usage, err := laptop.svc.Status.Usage(ctx)
	require.NoError(t, err)
	assert.Equal(t, []Usage{{AreaHot, "_shared", 1, 5}, {AreaStore, "web-1", 1, 2}}, usage)
}

func TestRemoveDevice(t *testing.T) {
	laptop, _, _ := twoDevices(t)
	_, err := laptop.svc.Shelves.Push(ctx, Shelf("web-1"), writeFile(t, t.TempDir(), "a", "1"), "")
	require.NoError(t, err)
	_, err = laptop.svc.Devices.Remove(ctx, "laptop", false)
	assert.ErrorContains(t, err, "cannot remove this device")
	res, err := laptop.svc.Devices.Remove(ctx, "web-1", true)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Purged)
	devs, _ := laptop.svc.Devices.List(ctx)
	assert.Len(t, devs, 1)
}

func TestPolicyScopesPrefix(t *testing.T) {
	p := SetupManager{}.Policy("my-hop", "team")
	assert.Contains(t, p, `"arn:aws:s3:::my-hop/team/*"`)
	assert.Contains(t, p, `"s3:ListBucket"`)
	assert.False(t, strings.Contains(p, "s3:*"))
}
