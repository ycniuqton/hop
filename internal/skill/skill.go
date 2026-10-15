// Package skill ships hop's guidance for coding agents, embedded so it always matches the binary.
package skill

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/ycniuqton/hop/internal/apperr"
)

//go:embed SKILL.md
var embedded string

// Source is skill text as checked out; a Windows checkout may have converted it to CRLF.
type Source string

// Normalized returns the text with LF line endings.
func (s Source) Normalized() string { return strings.ReplaceAll(string(s), "\r\n", "\n") }

var document = Source(embedded).Normalized()

// Document is the full SKILL.md, front matter included.
func Document() string { return document }

// Markers delimit the YAML front matter.
var Markers = struct{ FrontMatter string }{FrontMatter: "---\n"}

// Guidance is the skill without front matter, for AGENTS.md and agents without skill support.
func Guidance() string {
	rest := strings.TrimPrefix(document, Markers.FrontMatter)
	if _, body, ok := strings.Cut(rest, "\n"+Markers.FrontMatter); ok {
		return strings.TrimLeft(body, "\n")
	}
	return document
}

// Scope says whose skills folder receives the skill.
type Scope string

const (
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
)

// Layout is where Claude Code looks for skills.
var Layout = struct{ ConfigDir, SkillsDir, Name, File string }{
	ConfigDir: ".claude", SkillsDir: "skills", Name: "hop", File: "SKILL.md",
}

// Target resolves the skill path for one scope.
type Target interface {
	Path(home, configDir, workdir string) string
}

type userTarget struct{}

func (userTarget) Path(home, configDir, _ string) string {
	if configDir == "" {
		configDir = filepath.Join(home, Layout.ConfigDir)
	}
	return filepath.Join(configDir, Layout.SkillsDir, Layout.Name, Layout.File)
}

type projectTarget struct{}

func (projectTarget) Path(_, _, workdir string) string {
	return filepath.Join(workdir, Layout.ConfigDir, Layout.SkillsDir, Layout.Name, Layout.File)
}

// Targets is the scope registry; adding a destination means adding an entry here.
var Targets = map[Scope]Target{ScopeUser: userTarget{}, ScopeProject: projectTarget{}}

// Install writes the skill atomically; running it again rewrites the same bytes.
func Install(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return apperr.New(apperr.ExitGeneral, "cannot create %s: %v", filepath.Dir(path), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+Layout.File+"-*")
	if err != nil {
		return apperr.New(apperr.ExitGeneral, "cannot write %s: %v", path, err)
	}
	defer os.Remove(tmp.Name())
	_, werr := tmp.WriteString(document)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		return apperr.New(apperr.ExitGeneral, "cannot write %s: %v", path, werr)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return apperr.New(apperr.ExitGeneral, "cannot write %s: %v", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return apperr.New(apperr.ExitGeneral, "cannot write %s: %v", path, err)
	}
	return nil
}
