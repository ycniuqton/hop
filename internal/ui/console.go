// Package ui is hop's terminal surface: output, tables, prompts and progress bars.
package ui

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"github.com/ycniuqton/hop/internal/apperr"
)

// Console carries the streams and output mode of one command run.
type Console struct {
	In     io.Reader
	Out    io.Writer
	Err    io.Writer
	InTTY  bool
	ErrTTY bool
	Quiet  bool
	JSON   bool
	Color  bool
	Now    func() time.Time
	reader *bufio.Reader
}

// Style holds ANSI sequences and markers used for output.
var Style = struct{ Dim, Reset, Success, PlainSuccess string }{
	Dim: "\x1b[2m", Reset: "\x1b[0m", Success: "✔ ", PlainSuccess: "ok: ",
}

// NewConsole detects TTYs on the standard streams.
func NewConsole(in io.Reader, out, errw io.Writer) *Console {
	var t terminal
	return &Console{In: in, Out: out, Err: errw, InTTY: t.is(in), ErrTTY: t.is(errw) && t.prepare(errw), Color: t.is(out) && t.prepare(out), Now: time.Now}
}

// terminal detects whether a stream is an interactive terminal.
type terminal struct{}

func (terminal) is(v any) bool {
	f, ok := v.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// Printf writes a human line to stdout unless quiet or JSON mode is on.
func (c *Console) Printf(format string, args ...any) {
	if !c.Quiet && !c.JSON {
		fmt.Fprintf(c.Out, format+"\n", args...)
	}
}

// Successf writes a line confirming a finished action: ✔ on color terminals, "ok:" otherwise.
func (c *Console) Successf(format string, args ...any) {
	mark := Style.PlainSuccess
	if c.Color {
		mark = Style.Success
	}
	c.Printf(mark+format, args...)
}

// Warnf writes a warning to stderr unless JSON mode is on.
func (c *Console) Warnf(format string, args ...any) {
	if !c.JSON {
		fmt.Fprintf(c.Err, "warning: "+format+"\n", args...)
	}
}

// PrintJSON writes one JSON document to stdout.
func (c *Console) PrintJSON(v any) error {
	return c.PrintJSONTo(c.Out, v)
}

// PrintJSONTo writes one indented JSON document to w.
func (c *Console) PrintJSONTo(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// Outcome is what an action command did: Action names it, Data holds snake_case details.
type Outcome struct {
	Action Action
	Data   map[string]any
}

// Action names an action command's result in JSON output.
type Action string

// Actions are the action names reported with --json.
var Actions = struct {
	Init, Join, Rename, Remove, Rotate, Push, Pull, Keep, HotRemove, HotClear, Copy, Move, Delete, InstallCompletion, InstallSkill, Update, UpdateCheck, Uninstall Action
}{
	Init: "init", Join: "join", Rename: "rename", Remove: "remove", Rotate: "rotate", Push: "push", Pull: "pull", Keep: "keep",
	HotRemove: "hot_rm", HotClear: "hot_clear", Copy: "cp", Move: "mv", Delete: "rm",
	InstallCompletion: "completion_install", InstallSkill: "skill_install",
	Update: "update", UpdateCheck: "update_check", Uninstall: "uninstall",
}

// Done reports a finished action: the "✔ …" line, or with --json one JSON object.
func (c *Console) Done(o Outcome, format string, args ...any) error {
	c.Successf(format, args...)
	return c.Report(o)
}

// Report prints the outcome as one JSON object when --json is on, and nothing otherwise.
func (c *Console) Report(o Outcome) error {
	if !c.JSON {
		return nil
	}
	doc := map[string]any{"action": o.Action}
	for k, v := range o.Data {
		doc[k] = v
	}
	return c.PrintJSON(doc)
}

// Table prints aligned columns with a dim header on color terminals; columns are laid out before
// the header is dimmed, so the escape codes never count as width.
func (c *Console) Table(headers []string, rows [][]string) {
	if c.Quiet {
		return
	}
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, r := range rows {
		fmt.Fprintln(w, strings.Join(r, "\t"))
	}
	_ = w.Flush()
	header, body, _ := strings.Cut(buf.String(), "\n")
	if c.Color {
		header = Style.Dim + header + Style.Reset
	}
	fmt.Fprint(c.Out, header+"\n"+body)
}

// CanPrompt is true only when stdin is a terminal.
func (c *Console) CanPrompt() bool { return c.InTTY }

func (c *Console) line() (string, error) {
	if c.reader == nil {
		c.reader = bufio.NewReader(c.In)
	}
	s, err := c.reader.ReadString('\n')
	if err != nil && s == "" {
		return "", apperr.New(apperr.ExitUsage, "no input")
	}
	return strings.TrimSpace(s), nil
}

// ReadLine reads one trimmed line of input without prompting (for --secret-stdin and piped codes).
func (c *Console) ReadLine() (string, error) {
	return c.line()
}

// Ask prompts for a value with a default; without a TTY the missing value is a usage error.
func (c *Console) Ask(label, def, flag string) (string, error) {
	if !c.CanPrompt() {
		if def != "" {
			return def, nil
		}
		return "", apperr.New(apperr.ExitUsage, "missing %s", label).WithHint("pass %s", flag)
	}
	if def != "" {
		fmt.Fprintf(c.Err, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(c.Err, "%s: ", label)
	}
	v, err := c.line()
	if err != nil || v == "" {
		return def, err
	}
	return v, nil
}

// AskSecret reads a value without echo; an existing default is kept on empty input and never shown.
func (c *Console) AskSecret(label, def, flag string) (string, error) {
	if !c.CanPrompt() {
		return c.Ask(label, def, flag)
	}
	fmt.Fprintf(c.Err, "%s%s: ", label, map[bool]string{true: " [from environment]", false: ""}[def != ""])
	v, err := c.secretLine()
	if err != nil {
		return "", err
	}
	if v != "" {
		return v, nil
	}
	return def, nil
}

// secretLine reads one line with echo off on a real terminal.
func (c *Console) secretLine() (string, error) {
	f, ok := c.In.(*os.File)
	if !ok {
		return c.line()
	}
	b, err := term.ReadPassword(int(f.Fd()))
	fmt.Fprintln(c.Err)
	return strings.TrimSpace(string(b)), err
}

// Confirm asks a yes/no question; without a TTY it fails and asks for -y.
func (c *Console) Confirm(question string, defaultYes bool) (bool, error) {
	if !c.CanPrompt() {
		return false, apperr.New(apperr.ExitUsage, "%s", strings.TrimSuffix(question, "?")+": confirmation required").WithHint("re-run with -y")
	}
	suffix := map[bool]string{true: " [Y/n] ", false: " [y/N] "}[defaultYes]
	fmt.Fprint(c.Err, question+suffix)
	v, err := c.line()
	if err != nil {
		return false, err
	}
	switch strings.ToLower(v) {
	case "":
		return defaultYes, nil
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// Choose returns the chosen index: an ↑/↓ menu on a real terminal, a numbered list on other
// interactive input (Enter takes def, a bad answer asks again), and def without a TTY.
func (c *Console) Choose(title string, items []string, def int) (int, error) {
	if !c.CanPrompt() {
		return def, nil
	}
	if f, ok := c.In.(*os.File); ok && c.ErrTTY {
		return c.chooseWithArrows(f, title, items, def)
	}
	fmt.Fprintln(c.Err, title)
	for i, it := range items {
		fmt.Fprintf(c.Err, "  %d) %s\n", i+1, it)
	}
	for {
		fmt.Fprintf(c.Err, "Choose [%d]: ", def+1)
		v, err := c.line()
		if err != nil || v == "" {
			return def, err
		}
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= len(items) {
			return n - 1, nil
		}
		fmt.Fprintf(c.Err, "error: enter a number from 1 to %d\n", len(items))
	}
}

// chooseWithArrows runs the ↑/↓ menu with the terminal in raw mode, restoring it afterwards.
func (c *Console) chooseWithArrows(f *os.File, title string, items []string, def int) (int, error) {
	state, err := term.MakeRaw(int(f.Fd()))
	if err != nil {
		return 0, apperr.New(apperr.ExitGeneral, "cannot read the terminal: %v", err)
	}
	defer func() { _ = term.Restore(int(f.Fd()), state) }()
	m := &Menu{Title: title, Items: items, Cur: def, Color: c.Color}
	return m.Run(f, c.Err)
}

// Pick shows a numbered list and returns the chosen index.
func (c *Console) Pick(title string, items []string) (int, error) {
	fmt.Fprintln(c.Err, title)
	for i, it := range items {
		fmt.Fprintf(c.Err, "  %d) %s\n", i+1, it)
	}
	fmt.Fprint(c.Err, "number: ")
	v, err := c.line()
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > len(items) {
		return 0, apperr.New(apperr.ExitUsage, "invalid choice %q", v)
	}
	return n - 1, nil
}
