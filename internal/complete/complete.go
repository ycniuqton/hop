package complete

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/naming"
)

// Source supplies completion data: cache-backed devices and shelves, live store listings.
type Source interface {
	Self() string
	Devices() []string
	ShelfNames(shelf string) []string
	StoreChildren(device, dir string) (dirs, files []string)
}

// Completer turns a Source into Cobra completion functions (spec section 7 table).
type Completer struct {
	Src Source
}

// Syntax is the @ notation as seen by completion.
var Syntax = struct{ Mark, Sep, Dir, Self string }{Mark: "@", Sep: ":", Dir: "/", Self: "me"}

// HotMode says what a hot command's plain positional arguments are.
type HotMode int

const (
	HotNames HotMode = iota
	HotFiles
	HotShelfOnly
)

const (
	noFiles = cobra.ShellCompDirectiveNoFileComp
	noSpace = cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
)

// Hot completes push/pull/hot arguments: @devices anywhere, shelf names or local files otherwise.
func (c Completer) Hot(mode HotMode) cobra.CompletionFunc {
	return func(_ *cobra.Command, args []string, word string) ([]string, cobra.ShellCompDirective) {
		if strings.HasPrefix(word, Syntax.Mark) {
			return c.filter(c.deviceMarks(), word), noFiles
		}
		switch mode {
		case HotFiles:
			return nil, cobra.ShellCompDirectiveDefault
		case HotNames:
			if c.plainArgs(args) > 0 {
				return nil, cobra.ShellCompDirectiveDefault
			}
			return c.filter(c.Src.ShelfNames(c.shelfOf(args)), word), noFiles
		default:
			return nil, noFiles
		}
	}
}

// HotRemove completes `hot rm`: every plain argument is a shelf name.
func (c Completer) HotRemove() cobra.CompletionFunc {
	return func(_ *cobra.Command, args []string, word string) ([]string, cobra.ShellCompDirective) {
		if strings.HasPrefix(word, Syntax.Mark) {
			return c.filter(c.deviceMarks(), word), noFiles
		}
		return c.filter(c.Src.ShelfNames(c.shelfOf(args)), word), noFiles
	}
}

// Store completes store paths; withLocal also offers local files (cp sources and destinations).
func (c Completer) Store(withLocal bool) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, word string) ([]string, cobra.ShellCompDirective) {
		if strings.HasPrefix(word, Syntax.Mark) {
			return c.remote(word), noSpace
		}
		roots := c.deviceRoots()
		if !withLocal {
			return c.filter(roots, word), noSpace
		}
		return append(LocalFiles{}.List(word), c.filter(roots, word)...), noSpace
	}
}

// DeviceNames completes bare device names for the first n positional arguments.
func (c Completer) DeviceNames(n int) cobra.CompletionFunc {
	return func(_ *cobra.Command, args []string, word string) ([]string, cobra.ShellCompDirective) {
		if len(args) >= n {
			return nil, noFiles
		}
		return c.filter(c.Src.Devices(), word), noFiles
	}
}

func (c Completer) remote(word string) []string {
	head, p, hasPath := strings.Cut(word, Syntax.Sep)
	if !hasPath {
		return c.filter(c.deviceRoots(), word)
	}
	device := c.resolve(strings.TrimPrefix(head, Syntax.Mark))
	dir := ""
	if i := strings.LastIndex(p, Syntax.Dir); i >= 0 {
		dir = p[:i+1]
	}
	dirs, files := c.Src.StoreChildren(device, dir)
	out := make([]string, 0, len(dirs)+len(files))
	for _, d := range dirs {
		out = append(out, head+Syntax.Sep+dir+d)
	}
	for _, f := range files {
		out = append(out, head+Syntax.Sep+dir+f)
	}
	return c.filter(out, word)
}

func (c Completer) deviceMarks() []string {
	out := []string{Syntax.Mark, Syntax.Mark + naming.SharedAlias}
	for _, d := range c.Src.Devices() {
		out = append(out, Syntax.Mark+d)
	}
	return out
}

func (c Completer) deviceRoots() []string {
	out := []string{Syntax.Mark + Syntax.Sep, Syntax.Mark + naming.SharedAlias + Syntax.Sep}
	for _, d := range c.Src.Devices() {
		out = append(out, Syntax.Mark+d+Syntax.Sep)
	}
	return out
}

func (c Completer) resolve(name string) string {
	if name == "" || name == Syntax.Self {
		return c.Src.Self()
	}
	return naming.Devices.Resolve(name)
}

func (c Completer) shelfOf(args []string) string {
	for _, a := range args {
		if strings.HasPrefix(a, Syntax.Mark) && !strings.Contains(a, Syntax.Sep) {
			return c.resolve(strings.TrimPrefix(a, Syntax.Mark))
		}
	}
	return naming.SharedShelf
}

func (c Completer) plainArgs(args []string) int {
	n := 0
	for _, a := range args {
		if !strings.HasPrefix(a, Syntax.Mark) {
			n++
		}
	}
	return n
}

func (c Completer) filter(items []string, word string) []string {
	var out []string
	for _, it := range items {
		if strings.HasPrefix(it, word) {
			out = append(out, it)
		}
	}
	return out
}

// LocalFiles lists local paths matching a partial word, folders with a trailing slash.
type LocalFiles struct{}

func (LocalFiles) List(word string) []string {
	dir, base := filepath.Split(word)
	readDir := dir
	if readDir == "" {
		readDir = "."
	}
	entries, err := os.ReadDir(readDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), base) || (strings.HasPrefix(e.Name(), ".") && !strings.HasPrefix(base, ".")) {
			continue
		}
		name := dir + e.Name()
		if e.IsDir() {
			name += Syntax.Dir
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
