package ui

import (
	"fmt"
	"io"

	"github.com/ycniuqton/hop/internal/apperr"
)

// Keys are the raw terminal bytes the menu understands.
var Keys = struct {
	Escape, Bracket, Up, Down, Enter, Newline, CtrlC, CtrlD, Prev, Next, FirstDigit byte
}{Escape: 0x1b, Bracket: '[', Up: 'A', Down: 'B', Enter: '\r', Newline: '\n', CtrlC: 0x03, CtrlD: 0x04, Prev: 'k', Next: 'j', FirstDigit: '1'}

// MenuStyle holds the menu's markers and ANSI cursor controls.
var MenuStyle = struct {
	Pointer, Blank, Accent, Hint, LineUp, ClearDown, ClearLine, NewLine string
}{
	Pointer: "❯ ", Blank: "  ", Accent: "\x1b[36m", Hint: "(↑/↓ to move, Enter to select)",
	LineUp: "\x1b[%dA", ClearDown: "\x1b[J", ClearLine: "\r\x1b[2K", NewLine: "\r\n",
}

// Menu is an arrow-key picker drawn in place; the terminal must already be in raw mode.
type Menu struct {
	Title string
	Items []string
	Cur   int
	Color bool
}

// Run draws the menu, handles keys until a choice is made, and returns the chosen index.
func (m *Menu) Run(in io.Reader, out io.Writer) (int, error) {
	fmt.Fprint(out, m.header()+MenuStyle.NewLine)
	m.drawItems(out)
	for {
		done, err := m.press(in)
		if err != nil {
			return 0, err
		}
		if done {
			m.collapse(out)
			return m.Cur, nil
		}
		fmt.Fprintf(out, MenuStyle.LineUp, len(m.Items))
		m.drawItems(out)
	}
}

func (m *Menu) header() string {
	hint := MenuStyle.Hint
	if m.Color {
		hint = Style.Dim + hint + Style.Reset
	}
	return m.Title + " " + hint
}

func (m *Menu) drawItems(out io.Writer) {
	for i, it := range m.Items {
		line := MenuStyle.Blank + it
		if i == m.Cur {
			line = m.highlight(MenuStyle.Pointer + it)
		}
		fmt.Fprint(out, MenuStyle.ClearLine+MenuStyle.Blank+line+MenuStyle.NewLine)
	}
}

func (m *Menu) highlight(s string) string {
	if !m.Color {
		return s
	}
	return MenuStyle.Accent + s + Style.Reset
}

// collapse replaces the menu with one line: "Title Answer".
func (m *Menu) collapse(out io.Writer) {
	fmt.Fprintf(out, MenuStyle.LineUp, len(m.Items)+1)
	fmt.Fprint(out, MenuStyle.ClearLine+MenuStyle.ClearDown+m.Title+" "+m.highlight(m.Items[m.Cur])+MenuStyle.NewLine)
}

// press reads one key; it reports true when the current item is chosen.
func (m *Menu) press(in io.Reader) (bool, error) {
	b, err := m.readByte(in)
	if err != nil {
		return false, err
	}
	n := len(m.Items)
	switch {
	case b == Keys.Enter || b == Keys.Newline:
		return true, nil
	case b == Keys.CtrlC || b == Keys.CtrlD:
		return false, apperr.New(apperr.ExitInterrupted, "interrupted")
	case b >= Keys.FirstDigit && int(b-Keys.FirstDigit) < n:
		m.Cur = int(b - Keys.FirstDigit)
		return true, nil
	case b == Keys.Prev:
		m.Cur = (m.Cur + n - 1) % n
	case b == Keys.Next:
		m.Cur = (m.Cur + 1) % n
	case b == Keys.Escape:
		return false, m.arrow(in)
	}
	return false, nil
}

// arrow finishes an "ESC [ A/B" sequence; other escape sequences are ignored.
func (m *Menu) arrow(in io.Reader) error {
	seq := make([]byte, 0, 2)
	for len(seq) < 2 {
		b, err := m.readByte(in)
		if err != nil {
			return err
		}
		seq = append(seq, b)
	}
	n := len(m.Items)
	switch string(seq) {
	case string([]byte{Keys.Bracket, Keys.Up}):
		m.Cur = (m.Cur + n - 1) % n
	case string([]byte{Keys.Bracket, Keys.Down}):
		m.Cur = (m.Cur + 1) % n
	}
	return nil
}

// readByte reads exactly one byte, unbuffered, so input meant for later prompts stays unread.
func (m *Menu) readByte(in io.Reader) (byte, error) {
	var b [1]byte
	if _, err := io.ReadFull(in, b[:]); err != nil {
		return 0, apperr.New(apperr.ExitUsage, "no input")
	}
	return b[0], nil
}
