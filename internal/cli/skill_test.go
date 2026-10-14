package cli

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/skill"
	"github.com/ycniuqton/hop/internal/ui"
)

func TestSkillInstall(t *testing.T) {
	m := newMemoryHarness(t).machine("x")
	path := m.path(".claude/skills/hop/SKILL.md")
	assert.Equal(t, "ok: installed "+path+" — start a new Claude Code session to use it\n", m.ok("skill", "install"))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, skill.Document(), string(got))
	m.ok("skill", "install")
	again, _ := os.ReadFile(path)
	assert.Equal(t, got, again, "idempotent")
	entries, _ := os.ReadDir(filepath.Dir(path))
	assert.Len(t, entries, 1, "no temp files left behind")

	m.vars[EnvVars.ClaudeConfig] = m.path("custom-claude")
	doc := jsonDoc(t, m.ok("skill", "install", "--json"))
	assert.Equal(t, m.path("custom-claude/skills/hop/SKILL.md"), doc["path"])
	assert.Equal(t, "user", doc["scope"])

	project := t.TempDir()
	t.Chdir(project)
	doc = jsonDoc(t, m.ok("skill", "install", "--project", "--json"))
	assert.Equal(t, filepath.Join(project, ".claude/skills/hop/SKILL.md"), doc["path"])
}

func TestSkillPrintHasNoFrontMatter(t *testing.T) {
	out := newMemoryHarness(t).machine("x").ok("skill", "print")
	assert.True(t, strings.HasPrefix(out, "# hop"), out[:40])
	assert.NotContains(t, out, "description:")
	assert.True(t, strings.HasPrefix(skill.Document(), "---\nname: hop\ndescription: "))
}

// skillCommandLine finds hop invocations in the skill: inline code, code lines and ssh one-liners.
var skillCommandLine = regexp.MustCompile("(?m)(?:`|^\\s*(?:[A-Z_]+=\\S+\\s+)*(?:\\\\\\s*)?|'|#\\s+\\w+(?:\\s\\w+)?:\\s)hop ([a-z]+)(?: ([a-z]+))?([^`'\\n]*)")

// TestSkillMatchesCLI fails when the skill names a command or flag the CLI does not have.
func TestSkillMatchesCLI(t *testing.T) {
	root := NewApp(&Env{Console: ui.NewConsole(strings.NewReader(""), io.Discard, io.Discard)}).Root()
	matches := skillCommandLine.FindAllStringSubmatch(skill.Document(), -1)
	require.Greater(t, len(matches), 15, "the pattern should find the skill's examples")
	for _, mt := range matches {
		cmd := skillFind(t, root, mt[1], mt[2])
		for _, flag := range regexp.MustCompile(`--([a-z][a-z-]*)`).FindAllStringSubmatch(mt[3], -1) {
			found := cmd.Flags().Lookup(flag[1]) != nil || cmd.InheritedFlags().Lookup(flag[1]) != nil
			assert.True(t, found, "skill uses %q on 'hop %s %s', which has no such flag", flag[0], mt[1], mt[2])
		}
	}
}

func skillFind(t *testing.T, root *cobra.Command, name, sub string) *cobra.Command {
	t.Helper()
	cmd, _, err := root.Find([]string{name, sub})
	require.NoError(t, err)
	require.NotEqual(t, root, cmd, "skill mentions unknown command 'hop %s'", name)
	return cmd
}
