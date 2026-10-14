package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/store"
	"github.com/ycniuqton/hop/internal/target"
)

func (d device) targets(t *testing.T, args ...string) []target.Target {
	l, err := d.p.ParseAll(args)
	require.NoError(t, err)
	return l
}

func (d device) names(t *testing.T, arg string, recursive bool) []string {
	entries, err := d.svc.Store.List(ctx, d.target(t, arg), recursive)
	require.NoError(t, err)
	var out []string
	for _, e := range entries {
		out = append(out, e.Name)
	}
	return out
}

func TestCopyDestinationRules(t *testing.T) {
	laptop, _, _ := twoDevices(t)
	dir := t.TempDir()
	db := writeFile(t, dir, "db.sql", "data")

	_, err := laptop.svc.Store.Copy(ctx, laptop.targets(t, db), laptop.target(t, "@:backups/"), CopyOptions{})
	require.NoError(t, err)
	_, err = laptop.svc.Store.Copy(ctx, laptop.targets(t, db), laptop.target(t, "@web-1:backups/2026-10.sql"), CopyOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"db.sql"}, laptop.names(t, "@:backups/", false))
	assert.Equal(t, []string{"2026-10.sql"}, laptop.names(t, "@web-1:backups", false))

	_, err = laptop.svc.Store.Copy(ctx, laptop.targets(t, db), laptop.target(t, "@:backups/"), CopyOptions{})
	assert.EqualError(t, err, "exists: @laptop:backups/db.sql (use -f to overwrite)")
	_, err = laptop.svc.Store.Copy(ctx, laptop.targets(t, db), laptop.target(t, "@:backups/"), CopyOptions{Force: true})
	assert.NoError(t, err)

	_, err = laptop.svc.Store.Copy(ctx, laptop.targets(t, db, db), laptop.target(t, "@:x.sql"), CopyOptions{})
	assert.ErrorContains(t, err, "needs a folder destination")
	_, err = laptop.svc.Store.Copy(ctx, laptop.targets(t, db), target.Target{Kind: target.Local, Path: dir}, CopyOptions{})
	assert.EqualError(t, err, "use the regular cp for local copies")
}

func TestCopyRecursiveRoundTrip(t *testing.T) {
	laptop, web, _ := twoDevices(t)
	logs := filepath.Join(t.TempDir(), "logs")
	writeFile(t, logs, "a.log", "a")
	writeFile(t, logs, "sub/b.log", "b")

	_, err := laptop.svc.Store.Copy(ctx, laptop.targets(t, logs), laptop.target(t, "@web-1:logs/"), CopyOptions{})
	assert.ErrorContains(t, err, "is a directory — use -r")
	res, err := laptop.svc.Store.Copy(ctx, laptop.targets(t, logs), laptop.target(t, "@web-1:logs/"), CopyOptions{Recursive: true})
	require.NoError(t, err)
	assert.Equal(t, 2, res.Files)
	assert.Equal(t, []string{"logs/a.log", "logs/sub/b.log"}, laptop.names(t, "@web-1:logs/", true))
	assert.Equal(t, []string{"logs/"}, laptop.names(t, "@web-1:", false))

	out := t.TempDir()
	_, err = web.svc.Store.Copy(ctx, web.targets(t, "@:logs/logs"), target.Target{Kind: target.Local, Path: out}, CopyOptions{Recursive: true})
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(out, "logs", "sub", "b.log"))
	require.NoError(t, err)
	assert.Equal(t, "b", string(data))

	_, err = web.svc.Store.Copy(ctx, web.targets(t, "@web-1:logs/logs/a.log"), laptop.target(t, "@laptop:restore/"), CopyOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"a.log"}, laptop.names(t, "@:restore/", false))
}

func TestCopyDownloadSingleFileExactName(t *testing.T) {
	laptop, _, _ := twoDevices(t)
	_, err := laptop.svc.Store.Copy(ctx, laptop.targets(t, writeFile(t, t.TempDir(), "x.sql", "q")), laptop.target(t, "@:x.sql"), CopyOptions{})
	require.NoError(t, err)
	dest := filepath.Join(t.TempDir(), "renamed.sql")
	_, err = laptop.svc.Store.Copy(ctx, laptop.targets(t, "@:x.sql"), target.Target{Kind: target.Local, Path: dest}, CopyOptions{})
	require.NoError(t, err)
	data, _ := os.ReadFile(dest)
	assert.Equal(t, "q", string(data))
}

func TestDownloadRefusesEscapingKeys(t *testing.T) {
	laptop, _, fake := twoDevices(t)
	require.NoError(t, fake.Put(ctx, laptop.ws.Layout.StoreKey("laptop", "evil/../../x"), strings.NewReader("x"), 1, store.PutOptions{}))
	_, err := laptop.svc.Store.Copy(ctx, laptop.targets(t, "@:evil/"), target.Target{Kind: target.Local, Path: t.TempDir()}, CopyOptions{Recursive: true})
	assert.ErrorContains(t, err, "refusing to write outside")
}

func TestMoveAndRemove(t *testing.T) {
	laptop, _, _ := twoDevices(t)
	dir := t.TempDir()
	_, err := laptop.svc.Store.Copy(ctx, laptop.targets(t, writeFile(t, dir, "a.sql", "a")), laptop.target(t, "@web-1:"), CopyOptions{})
	require.NoError(t, err)
	_, err = laptop.svc.Store.Move(ctx, laptop.target(t, "@web-1:a.sql"), laptop.target(t, "@web-1:old/a.sql"), false)
	require.NoError(t, err)
	assert.Equal(t, []string{"old/"}, laptop.names(t, "@web-1:", false))

	_, err = laptop.svc.Store.Remove(ctx, laptop.targets(t, "@web-1:old/"), false, nil)
	assert.ErrorContains(t, err, "is a folder — use -r")
	n, err := laptop.svc.Store.Remove(ctx, laptop.targets(t, "@web-1:old/"), true, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	_, err = laptop.svc.Store.Remove(ctx, laptop.targets(t, "@web-1:nope"), false, nil)
	assert.ErrorContains(t, err, "no such path: @web-1:nope")
}

func TestRemoveConfirmThreshold(t *testing.T) {
	laptop, _, _ := twoDevices(t)
	dir := filepath.Join(t.TempDir(), "many")
	for i := 0; i < 11; i++ {
		writeFile(t, dir, string(rune('a'+i)), "x")
	}
	_, err := laptop.svc.Store.Copy(ctx, laptop.targets(t, dir), laptop.target(t, "@:"), CopyOptions{Recursive: true})
	require.NoError(t, err)

	asked := 0
	n, err := laptop.svc.Store.Remove(ctx, laptop.targets(t, "@:many/"), true, func(n int) (bool, error) { asked = n; return false, nil })
	require.NoError(t, err)
	assert.Equal(t, 11, asked)
	assert.Equal(t, 0, n, "declined")
	n, err = laptop.svc.Store.Remove(ctx, laptop.targets(t, "@:many/"), true, func(int) (bool, error) { return true, nil })
	require.NoError(t, err)
	assert.Equal(t, 11, n)
}

func TestRenameMovesShelfAndStore(t *testing.T) {
	laptop, web, _ := twoDevices(t)
	_, err := laptop.svc.Shelves.Push(ctx, Shelf("web-1"), writeFile(t, t.TempDir(), "h", "1"), "")
	require.NoError(t, err)
	_, err = laptop.svc.Store.Copy(ctx, laptop.targets(t, writeFile(t, t.TempDir(), "s", "2")), laptop.target(t, "@web-1:dir/"), CopyOptions{})
	require.NoError(t, err)

	res, err := web.svc.Devices.Rename(ctx, "web-1", "web-2")
	require.NoError(t, err)
	assert.Equal(t, RenameResult{HotObjects: 1, StoreObjects: 1, Self: true}, res)
	assert.Equal(t, "web-2", web.ws.Self)

	_, err = laptop.svc.Devices.Get(ctx, "web-1")
	assert.ErrorContains(t, err, "unknown device")
	files, _ := laptop.svc.Shelves.List(ctx, Shelf("web-2"), false)
	assert.Len(t, files, 1)
	assert.Equal(t, []string{"dir/s"}, laptop.names(t, "@web-2:", true))

	_, err = laptop.svc.Devices.Rename(ctx, "web-2", "laptop")
	assert.ErrorContains(t, err, "already used")
}
