// Package complete implements tab completion: candidate functions (cache-only, except live store paths) and shell installers.
package complete

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/apperr"
)

// Shell is a supported completion shell.
type Shell string

const (
	Bash Shell = "bash"
	Zsh  Shell = "zsh"
	Fish Shell = "fish"
)

// Installer knows where one shell loads completions from and how to generate the script.
type Installer interface {
	Path(home string) string
	Script(root *cobra.Command, w io.Writer) error
	Hint(path string, getenv func(string) string) string
}

type bashInstaller struct{}

func (bashInstaller) Path(home string) string {
	return filepath.Join(home, ".local", "share", "bash-completion", "completions", "hop")
}
func (bashInstaller) Script(root *cobra.Command, w io.Writer) error {
	return root.GenBashCompletionV2(w, true)
}
func (bashInstaller) Hint(string, func(string) string) string { return "" }

type zshInstaller struct{}

func (zshInstaller) Path(home string) string {
	return filepath.Join(home, ".zsh", "completions", "_hop")
}
func (zshInstaller) Script(root *cobra.Command, w io.Writer) error {
	return root.GenZshCompletion(w)
}
func (zshInstaller) Hint(path string, getenv func(string) string) string {
	dir := filepath.Dir(path)
	for _, p := range strings.Split(getenv("FPATH"), ":") {
		if p == dir {
			return ""
		}
	}
	return "add this line to ~/.zshrc before compinit: fpath=(" + dir + " $fpath)"
}

type fishInstaller struct{}

func (fishInstaller) Path(home string) string {
	return filepath.Join(home, ".config", "fish", "completions", "hop.fish")
}
func (fishInstaller) Script(root *cobra.Command, w io.Writer) error {
	return root.GenFishCompletion(w, true)
}
func (fishInstaller) Hint(string, func(string) string) string { return "" }

// Installers is the shell registry; adding a shell means adding an entry here.
var Installers = map[Shell]Installer{Bash: bashInstaller{}, Zsh: zshInstaller{}, Fish: fishInstaller{}}

// Shells lists supported shells in a stable order.
func Shells() []string { return []string{string(Bash), string(Zsh), string(Fish)} }

// Lookup returns the installer for a shell name.
func Lookup(name string) (Shell, Installer, error) {
	sh := Shell(name)
	inst, ok := Installers[sh]
	if !ok {
		return "", nil, apperr.New(apperr.ExitUsage, "unsupported shell %q", name).
			WithHint("use one of: %s", strings.Join(Shells(), ", "))
	}
	return sh, inst, nil
}

// Detect picks the shell from $SHELL.
func Detect(shellPath string) (Shell, Installer, error) {
	if shellPath == "" {
		return "", nil, apperr.New(apperr.ExitUsage, "cannot detect your shell ($SHELL is empty)").
			WithHint("run 'hop completion <%s>' and install the script yourself", strings.Join(Shells(), "|"))
	}
	return Lookup(filepath.Base(shellPath))
}

// Install writes the script; rewriting identical content is a no-op, so it is idempotent.
func Install(root *cobra.Command, inst Installer, home string) (string, error) {
	path := inst.Path(home)
	var buf bytes.Buffer
	if err := inst.Script(root, &buf); err != nil {
		return "", err
	}
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, buf.Bytes()) {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", apperr.New(apperr.ExitGeneral, "cannot create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return "", apperr.New(apperr.ExitGeneral, "cannot write %s: %v", path, err)
	}
	return path, nil
}
