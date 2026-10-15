package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
)

// recordingSpawner remembers which background refreshes completion asked for.
type recordingSpawner struct {
	calls []string
}

func (r *recordingSpawner) Spawn(what RefreshTarget, args ...string) {
	r.calls = append(r.calls, strings.Join(append([]string{string(what)}, args...), " "))
}

// completion is what `hop __completeNoDesc` printed: candidates plus the shell directive.
type completion struct {
	items     []string
	directive cobra.ShellCompDirective
}

func (m *machine) complete(args ...string) completion {
	m.h.t.Helper()
	out := m.ok(append([]string{cobra.ShellCompNoDescRequestCmd}, args...)...)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	last := lines[len(lines)-1]
	require.True(m.h.t, strings.HasPrefix(last, ":"), "no directive line in %q", out)
	var d int
	for _, ch := range strings.TrimPrefix(last, ":") {
		d = d*10 + int(ch-'0')
	}
	return completion{items: lines[:len(lines)-1], directive: cobra.ShellCompDirective(d)}
}

const (
	dirNoFiles = cobra.ShellCompDirectiveNoFileComp
	dirNoSpace = cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
)

// completionWorld is laptop + web-1 with names on two shelves and files in web-1's store.
func completionWorld(t *testing.T) (*machine, *machine) {
	a, b := newMemoryHarness(t).pair()
	a.ok("devices")
	a.ok("push", a.file("dump.sql", "x"))
	a.ok("push", a.file("site.tar", "x"), "@web-1")
	b.ok("cp", b.file("db.sql", "x"), "@:backups/db.sql")
	b.ok("cp", b.file("notes.txt", "x"), "@:notes.txt")
	return a, b
}

// TestCompletionTable covers every row of spec section 7 "What completes where".
func TestCompletionTable(t *testing.T) {
	a, _ := completionWorld(t)
	rows := []struct {
		name      string
		args      []string
		want      []string
		directive cobra.ShellCompDirective
	}{
		{"command tree", []string{"pu"}, []string{"pull", "push"}, dirNoFiles},
		{"push local files", []string{"push", ""}, nil, cobra.ShellCompDirectiveDefault},
		{"@ in a hot command", []string{"push", "dump.sql", "@"}, []string{"@", "@laptop", "@web-1"}, dirNoFiles},
		{"@ prefix narrows", []string{"pull", "@w"}, []string{"@web-1"}, dirNoFiles},
		{"pull shared names", []string{"pull", ""}, []string{"dump.sql"}, dirNoFiles},
		{"hot rm shared names", []string{"hot", "rm", ""}, []string{"dump.sql"}, dirNoFiles},
		{"pull with @web-1 on the line", []string{"pull", "@web-1", ""}, []string{"site.tar"}, dirNoFiles},
		{"hot rm with @web-1 on the line", []string{"hot", "rm", "@web-1", ""}, []string{"site.tar"}, dirNoFiles},
		{"live store listing", []string{"ls", "@web-1:"}, []string{"@web-1:backups/", "@web-1:notes.txt"}, dirNoSpace},
		{"store prefix", []string{"cp", "@web-1:ba"}, []string{"@web-1:backups/"}, dirNoSpace},
		{"store deeper", []string{"rm", "@web-1:backups/"}, []string{"@web-1:backups/db.sql"}, dirNoSpace},
		{"mv store paths", []string{"mv", "@web-1:n"}, []string{"@web-1:notes.txt"}, dirNoSpace},
		{"own store via @:", []string{"ls", "@:"}, nil, dirNoSpace},
		{"device roots", []string{"ls", "@"}, []string{"@:", "@laptop:", "@web-1:"}, dirNoSpace},
		{"rename device names", []string{"rename", ""}, []string{"laptop", "web-1"}, dirNoFiles},
		{"remove device names", []string{"remove", "w"}, []string{"web-1"}, dirNoFiles},
		{"rename second arg", []string{"rename", "web-1", ""}, nil, dirNoFiles},
	}
	for _, r := range rows {
		got := a.complete(r.args...)
		assert.Equal(t, r.want, nilIfEmpty(got.items), r.name)
		assert.Equal(t, r.directive, got.directive, r.name)
	}
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

func TestCompletionCpOffersLocalFilesAndDevices(t *testing.T) {
	a, _ := completionWorld(t)
	got := a.complete("cp", "db.sql", "")
	assert.Equal(t, dirNoSpace, got.directive)
	assert.Contains(t, got.items, "@:")
	assert.Contains(t, got.items, "@web-1:")
	assert.Contains(t, got.items, "completion_test.go", "local files from the working directory")

	dir := a.path("") + string(filepath.Separator)
	got = a.complete("cp", "db.sql", dir+"du")
	assert.Equal(t, []string{dir + "dump.sql"}, got.items)
}

func TestCompletionStaleCacheSpawnsRefresh(t *testing.T) {
	a, _ := completionWorld(t)
	rec := &recordingSpawner{}
	a.spawner = rec

	a.complete("push", "x", "@")
	assert.Empty(t, rec.calls, "fresh cache answers without a refresh")

	a.now = func() time.Time { return time.Now().Add(time.Minute) }
	got := a.complete("pull", "")
	assert.Equal(t, []string{"dump.sql"}, got.items, "stale cache still answers")
	assert.Equal(t, []string{"shelf _shared"}, rec.calls)

	require.NoError(t, os.RemoveAll(filepath.Join(a.home, ".cache")))
	a.now, rec.calls = time.Now, nil
	assert.Equal(t, []string{"@"}, a.complete("push", "x", "@").items)
	assert.Equal(t, []string{"devices"}, rec.calls)

	a.ok("__refresh", "devices")
	a.ok("__refresh", "shelf", "web-1")
	rec.calls = nil
	assert.Equal(t, []string{"@", "@laptop", "@web-1"}, a.complete("push", "x", "@").items)
	assert.Equal(t, []string{"site.tar"}, a.complete("pull", "@web-1", "").items)
	assert.Empty(t, rec.calls)
}

func TestCompletionWriteThrough(t *testing.T) {
	a, b := completionWorld(t)
	b.ok("pull", "site.tar", "@", b.path("site.tar"), "--rm")
	assert.Empty(t, b.complete("pull", "@", "").items, "pull --rm drops the name locally")
	a.ok("hot", "rm", "dump.sql")
	assert.Empty(t, a.complete("pull", "").items, "hot rm drops the name locally")
}

func TestCompletionWithoutConfigIsSilent(t *testing.T) {
	m := newMemoryHarness(t).machine("x")
	assert.Equal(t, []string{"@"}, m.complete("push", "x", "@").items)
	assert.Empty(t, m.complete("pull", "").items)
	assert.Empty(t, m.complete("ls", "@web-1:").items)
}

func TestCompletionInstall(t *testing.T) {
	m := newMemoryHarness(t).machine("x")
	m.vars[EnvVars.Shell] = "/bin/bash"
	bashPath := m.path(".local/share/bash-completion/completions/hop")
	assert.Equal(t, "ok: installed "+bashPath+" — open a new shell to use it\n", m.ok("completion", "install"))
	first, err := os.ReadFile(bashPath)
	require.NoError(t, err)
	m.ok("completion", "install")
	again, _ := os.ReadFile(bashPath)
	assert.Equal(t, first, again, "install is idempotent")
	assert.Equal(t, string(first), m.ok("completion", "bash"))

	m.vars[EnvVars.Shell] = "/usr/bin/zsh"
	zshDir := m.path(".zsh/completions")
	assert.Contains(t, m.ok("completion", "install"), "fpath=("+zshDir+" $fpath)")
	if runtime.GOOS != "windows" {
		m.vars[EnvVars.FPath] = "/usr/share/zsh:" + zshDir
		assert.NotContains(t, m.ok("completion", "install"), "fpath=", "FPATH is colon-separated, which clashes with C:")
	}

	m.vars[EnvVars.Shell] = "/usr/bin/fish"
	assert.Contains(t, m.ok("completion", "install"), m.path(".config/fish/completions/hop.fish"))

	m.vars[EnvVars.Shell] = "/bin/tcsh"
	assert.Contains(t, m.fails(apperr.ExitUsage, "completion", "install"), `unsupported shell "tcsh"`)
	m.vars[EnvVars.Shell] = ""
	if runtime.GOOS == "windows" {
		assert.Contains(t, m.ok("completion", "install"), "completion.ps1", "Windows defaults to PowerShell")
		return
	}
	assert.Contains(t, m.fails(apperr.ExitUsage, "completion", "install"), "hop completion <bash|zsh|fish|powershell>")
}

func TestCompletionPowerShell(t *testing.T) {
	m := newMemoryHarness(t).machine("x")
	m.vars[EnvVars.Shell] = "pwsh"
	path := filepath.Join(m.home, "AppData", "Local", "hop", "completion.ps1")
	out := m.ok("completion", "install")
	assert.Contains(t, out, "ok: installed "+path)
	assert.Contains(t, out, `add this line to your PowerShell profile (notepad $PROFILE), then open a new terminal: . "`+path+`"`)
	script, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(script), "Register-ArgumentCompleter")
	assert.Equal(t, string(script), m.ok("completion", "powershell"))
}
