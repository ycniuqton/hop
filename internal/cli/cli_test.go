package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
)

func TestHelpListsEveryV1Command(t *testing.T) {
	out := newMemoryHarness(t).machine("x").ok("--help")
	for _, name := range []string{
		"init", "invite", "join", "devices", "whoami", "rename", "remove", "rotate", "status", "completion",
		"push", "pull", "hot", "keep", "cp", "ls", "mv", "rm",
	} {
		assert.Contains(t, out, "  "+name+" ", name)
	}
	assert.NotContains(t, out, "__refresh")
}

func TestUsageExitCodes(t *testing.T) {
	m := newMemoryHarness(t).machine("x")
	assert.Equal(t, "hop test\n", m.ok("--version"))
	m.fails(apperr.ExitUsage, "--nope")
	m.fails(apperr.ExitUsage, "nope")
	m.fails(apperr.ExitUsage, "push")
	m.fails(apperr.ExitUsage, "whoami", "extra")
	assert.Equal(t, "error: not configured — run 'hop init' or 'hop join <code>'", m.fails(apperr.ExitGeneral, "whoami"))
}

func TestSetupFlow(t *testing.T) {
	h := newMemoryHarness(t)
	a, b := h.pair()
	assert.Contains(t, a.ok("whoami"), "laptop")

	out := b.ok("devices")
	assert.Contains(t, out, "web-1 (you)")
	assert.Contains(t, out, "laptop")
	assert.Contains(t, out, "laptop.example.internal")

	c := h.machine("web-1")
	code := strings.TrimSpace(a.ok("invite", "--code"))
	assert.Equal(t, `error: device name "web-1" is already used — pick another with --name`, c.fails(apperr.ExitConflict, "join", code, "--name", "web-1"))
	r := c.run(code+"\n", "join", "--name", "db-1")
	require.Equal(t, apperr.ExitOK, r.code, r.err)
	assert.Equal(t, "Joined workspace as \"db-1\" (3 devices).\n", r.out)

	assert.Contains(t, a.fails(apperr.ExitConflict, "init", "--bucket", "my-hop"), "already configured")
	fresh := h.machine("other")
	assert.Equal(t, "error: bucket already has a hop workspace — run 'hop invite' on an existing device and use 'hop join' instead",
		fresh.fails(apperr.ExitConflict, "init", "--bucket", "my-hop", "--name", "other"))
	assert.Contains(t, b.fails(apperr.ExitConflict, "join", code), "use --force")

	info, err := os.Stat(a.path(".config/hop/config.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestInviteOutput(t *testing.T) {
	a, _ := newMemoryHarness(t).pair()
	out := a.ok("invite")
	assert.Contains(t, out, "contains your S3 keys — treat it like a password")
	assert.Contains(t, out, "  hop join hop1:")
	assert.NotContains(t, out, "test-secret-value")
}

func TestHotFlowAndErrors(t *testing.T) {
	a, b := newMemoryHarness(t).pair()
	src := a.file("dump.sql", "select 1;")
	a.ok("push", src, "@web-1")
	a.ok("push", src)

	assert.Equal(t, `error: unknown device "web1" — did you mean web-1?`, a.fails(apperr.ExitNotFound, "push", src, "@web1"))
	assert.Equal(t, `error: "dmp.sql" not on shared shelf — did you mean dump.sql?`, b.fails(apperr.ExitNotFound, "pull", "dmp.sql", b.path("x")))

	b.ok("pull", "@", "dump.sql", b.path("mine.sql"))
	data, _ := os.ReadFile(b.path("mine.sql"))
	assert.Equal(t, "select 1;", string(data))
	assert.Contains(t, b.fails(apperr.ExitConflict, "pull", "dump.sql", "@", b.path("mine.sql")), "use -f to overwrite")

	a.ok("hot", "rm", "dump.sql")
	assert.Equal(t, `error: "dump.sql" not on shared shelf — did you mean: hop pull dump.sql @`,
		b.fails(apperr.ExitNotFound, "pull", "dump.sql", b.path("y")))

	out := b.ok("hot", "@")
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "dump.sql")
	assert.Contains(t, out, "laptop")

	b.ok("pull", "dump.sql", "@", b.path("z"), "--rm")
	assert.Contains(t, b.ok("hot", "@"), "is empty")
	assert.Contains(t, b.fails(apperr.ExitUsage, "pull"), "missing file name")
	assert.Contains(t, a.fails(apperr.ExitUsage, "push", a.home), "push sends single files — use 'hop cp -r' for folders")
	assert.Contains(t, a.fails(apperr.ExitUsage, "push", src, "@", "@web-1"), "only one @device")
}

func TestHotClearNeedsYes(t *testing.T) {
	a, _ := newMemoryHarness(t).pair()
	a.ok("push", a.file("a", "1"))
	assert.Contains(t, a.fails(apperr.ExitUsage, "hot", "clear"), "re-run with -y")
	assert.Contains(t, a.ok("hot", "clear", "-y"), "deleted 1 file(s)")
}

func TestStoreFlow(t *testing.T) {
	a, b := newMemoryHarness(t).pair()
	a.file("logs/app.log", "a")
	a.file("logs/sub/x.log", "x")
	b.ok("cp", "-r", a.path("logs"), "@:")
	assert.Equal(t, "logs/app.log\nlogs/sub/x.log\n", a.ok("ls", "-R", "@web-1:"))
	assert.Equal(t, "logs/\n", b.ok("ls"))

	a.ok("cp", "@web-1:logs/app.log", "@laptop:restore/")
	assert.Equal(t, "error: exists: @laptop:restore/app.log (use -f to overwrite)",
		a.fails(apperr.ExitConflict, "cp", "@web-1:logs/app.log", "@laptop:restore/"))
	assert.Contains(t, a.fails(apperr.ExitUsage, "cp", a.path("logs/app.log"), "@web-1"), "did you mean @web-1: (with a colon)?")

	a.ok("cp", "@:restore/app.log", a.path("back.log"))
	data, _ := os.ReadFile(a.path("back.log"))
	assert.Equal(t, "a", string(data))

	a.ok("mv", "@web-1:logs/app.log", "@web-1:old/app.log")
	assert.Contains(t, a.ok("ls", "-l", "@web-1:"), "old/")
	a.ok("rm", "-r", "@web-1:old/")
	assert.Equal(t, "logs/\n", a.ok("ls", "@web-1:"))
}

func TestBridgesAndJSON(t *testing.T) {
	a, _ := newMemoryHarness(t).pair()
	a.ok("push", a.file("db.sql", "data"))
	assert.Equal(t, "kept db.sql → @web-1:backups/db.sql\n", a.ok("keep", "db.sql", "@web-1:backups/"))
	a.ok("push", "@web-1:backups/db.sql", "@", "--as", "restore.sql")

	var files []map[string]any
	require.NoError(t, json.Unmarshal([]byte(a.ok("hot", "@", "--json")), &files))
	require.Len(t, files, 1)
	assert.Equal(t, "restore.sql", files[0]["name"])

	var devices []map[string]any
	require.NoError(t, json.Unmarshal([]byte(a.ok("devices", "--json")), &devices))
	assert.Len(t, devices, 2)

	var usage []map[string]any
	require.NoError(t, json.Unmarshal([]byte(a.ok("status", "--json")), &usage))
	assert.NotEmpty(t, usage)
	assert.Equal(t, "[]\n", a.ok("ls", "--json", "@:"))
}

func TestRenameRemoveRotate(t *testing.T) {
	h := newMemoryHarness(t)
	a, b := h.pair()
	b.ok("push", b.file("x", "1"), "@")
	assert.Contains(t, b.ok("rename", "web-1", "web-2"), `Renamed "web-1" to "web-2" (moved 1 hot files`)
	assert.Contains(t, b.ok("whoami"), "web-2")
	assert.Contains(t, b.ok("hot", "@"), "x")

	assert.Contains(t, a.fails(apperr.ExitUsage, "remove", "web-2"), "-y")
	r := a.run("", "remove", "web-2", "-y", "--purge")
	require.Equal(t, apperr.ExitOK, r.code, r.err)
	assert.Contains(t, r.err, "still holds working S3 keys")

	code := strings.TrimSpace(a.ok("invite", "--code"))
	assert.Contains(t, a.ok("rotate", code), "Rotated credentials")
	other := h.machine("z")
	other.ok("init", "--bucket", "my-hop", "--prefix", "other", "--name", "z")
	assert.Contains(t, other.fails(apperr.ExitConflict, "rotate", code), "different workspace")
}

func TestConfigPermissionsEnforced(t *testing.T) {
	a, _ := newMemoryHarness(t).pair()
	require.NoError(t, os.Chmod(a.path(".config/hop/config.json"), 0o644))
	assert.Contains(t, a.fails(apperr.ExitAuth, "whoami"), "chmod 600")
}

func TestPrintPolicy(t *testing.T) {
	out := newMemoryHarness(t).machine("x").ok("init", "--print-policy", "--bucket", "my-hop")
	assert.Contains(t, out, "arn:aws:s3:::my-hop/*")
}
