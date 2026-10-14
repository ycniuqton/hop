package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BashCompletionEnv locates the tools the real-shell completion test needs.
var BashCompletionEnv = struct{ Bash, Helpers, Package string }{
	Bash: "bash", Helpers: "/usr/share/bash-completion/bash_completion", Package: "github.com/ycniuqton/hop/cmd/hop",
}

// bashColonScript drives the generated script the way bash does on TAB: ':' is a word break.
const bashColonScript = `
source "$HOP_HELPERS"
source <(hop completion bash)
COMP_LINE='hop ls @web-1:ba'
COMP_POINT=${#COMP_LINE}
COMP_WORDS=(hop ls @web-1 : ba)
COMP_CWORD=4
__start_hop hop ba :
printf '%s\n' "${COMPREPLY[@]}"
`

// TestBashCompletesPathAfterColon is the spec section 7 "bash colon gotcha" test, run on the real binary.
func TestBashCompletesPathAfterColon(t *testing.T) {
	bash, err := exec.LookPath(BashCompletionEnv.Bash)
	if err != nil || testing.Short() {
		t.Skip("bash not available or -short")
	}
	if _, err := os.Stat(BashCompletionEnv.Helpers); err != nil {
		t.Skip("bash-completion not installed")
	}
	h := newS3Harness(t)
	web := h.machine("web-1")
	web.ok(append([]string{"init", "--bucket", "my-hop", "--name", "web-1"}, h.initFlags...)...)
	web.ok("cp", web.file("db.sql", "x"), "@:backups/db.sql")

	binDir := t.TempDir()
	build := exec.Command("go", "build", "-o", filepath.Join(binDir, "hop"), BashCompletionEnv.Package)
	out, err := build.CombinedOutput()
	require.NoError(t, err, string(out))

	cmd := exec.Command(bash, "--noprofile", "--norc", "-c", bashColonScript)
	cmd.Env = append(os.Environ(), "HOME="+web.home, "XDG_CONFIG_HOME=", "XDG_CACHE_HOME=", "HOP_CONFIG=",
		"HOP_HELPERS="+BashCompletionEnv.Helpers, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, []string{"backups/"}, strings.Fields(string(got)),
		"bash keeps '@web-1:' and inserts the rest, giving '@web-1:backups/'")
}
