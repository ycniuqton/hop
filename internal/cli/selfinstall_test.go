package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/selfinstall/selfinstalltest"
)

func updateMachine(t *testing.T, version string) (*machine, *selfinstalltest.Server) {
	srv := selfinstalltest.Start(t, "me/hop", "v9.9.9", []byte("new-binary"))
	m := newMemoryHarness(t).machine("x")
	m.version = version
	m.vars[EnvVars.ReleaseBase], m.vars[EnvVars.Repo] = srv.URL, "me/hop"
	m.file(".local/bin/hop", "old-binary")
	return m, srv
}

func (m *machine) binaryBody() string {
	b, _ := os.ReadFile(m.binary)
	return string(b)
}

func TestUpdateReplacesBinary(t *testing.T) {
	m, _ := updateMachine(t, "0.2.0")
	assert.Equal(t, "ok: update available: 0.2.0 → 9.9.9 (run 'hop update')\n", m.ok("update", "--check"))
	assert.Equal(t, "old-binary", m.binaryBody(), "--check changes nothing")
	out := m.ok("update")
	assert.Contains(t, out, "ok: updated hop 0.2.0 → 9.9.9 ("+m.binary+")")
	assert.Contains(t, out, "hop skill install")
	assert.Equal(t, "new-binary", m.binaryBody())
	info, _ := os.Stat(m.binary)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}

func TestUpdateWhenCurrentOrNewer(t *testing.T) {
	m, srv := updateMachine(t, "9.9.9")
	assert.Equal(t, "ok: hop 9.9.9 is up to date\n", m.ok("update"))
	m.version = "10.0.0-dev"
	assert.Equal(t, "ok: hop 10.0.0-dev is newer than the latest release 9.9.9 (use --force to install 9.9.9)\n", m.ok("update"))
	assert.Zero(t, srv.Downloads)
	m.ok("update", "--force")
	assert.Equal(t, "new-binary", m.binaryBody())
}

func TestUpdatePinnedAndJSON(t *testing.T) {
	m, _ := updateMachine(t, "9.9.9")
	doc := jsonDoc(t, m.ok("update", "--version", "9.9.9", "--json"))
	assert.Equal(t, map[string]any{"action": "update", "current": "9.9.9", "latest": "9.9.9", "update_available": false, "path": m.binary}, doc)
	doc = jsonDoc(t, m.ok("update", "--check", "--json"))
	assert.Equal(t, "update_check", doc["action"])
	r := m.run("", "update", "--version", "v1.0.0", "--json")
	assert.Equal(t, "not_found", jsonDoc(t, r.err)["kind"])
}

func TestUpdateRefusesBadChecksum(t *testing.T) {
	m, srv := updateMachine(t, "0.1.0")
	srv.BadSum = true
	assert.Contains(t, m.fails(apperr.ExitGeneral, "update"), "checksum mismatch")
	assert.Equal(t, "old-binary", m.binaryBody())
}

// installedMachine has every footprint item: binary, completion, skill, cache and config.
func installedMachine(t *testing.T) *machine {
	a, _ := newMemoryHarness(t).pair()
	a.file(".local/bin/hop", "binary")
	a.vars[EnvVars.Shell] = "/bin/bash"
	a.ok("completion", "install")
	a.ok("skill", "install")
	a.ok("push", a.file("x", "x"))
	return a
}

func TestUninstallNeedsConfirmation(t *testing.T) {
	a := installedMachine(t)
	assert.Contains(t, a.fails(apperr.ExitUsage, "uninstall"), "re-run with -y")
	r := a.typingArgs([]string{"n"}, "uninstall")
	require.Equal(t, apperr.ExitOK, r.code, r.err)
	assert.FileExists(t, a.binary)
	assert.FileExists(t, a.path(".config/hop/config.json"))
}

func TestUninstallRemovesFootprint(t *testing.T) {
	a := installedMachine(t)
	out := a.ok("uninstall", "-y")
	for _, line := range []string{
		"hop will remove:", "  binary     " + a.binary, "  completion " + a.path(".local/share/bash-completion/completions/hop"),
		"  skill      " + a.path(".claude/skills/hop/SKILL.md"), "  cache      " + a.path(".cache/hop"),
		"  config     " + a.path(".config/hop/config.json"), "ok: removed 5 item(s); hop is uninstalled",
		`Device "laptop" is still listed in the workspace — run 'hop remove laptop' on another device.`,
	} {
		assert.Contains(t, out, line+"\n")
	}
	for _, p := range []string{a.binary, a.path(".config/hop"), a.path(".cache/hop"), a.path(".claude/skills/hop")} {
		assert.NoFileExists(t, p)
		assert.NoDirExists(t, p)
	}
	assert.Equal(t, "ok: nothing to remove\n", a.ok("uninstall", "-y"))
}

func TestUninstallKeepConfigAndJSON(t *testing.T) {
	a := installedMachine(t)
	doc := jsonDoc(t, a.ok("uninstall", "-y", "--keep-config", "--json"))
	assert.Equal(t, "uninstall", doc["action"])
	assert.Len(t, doc["removed"], 4)
	assert.FileExists(t, a.path(".config/hop/config.json"))
	assert.NotContains(t, a.ok("whoami", "--json"), "test-secret-value")
}
