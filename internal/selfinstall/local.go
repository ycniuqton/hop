package selfinstall

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ycniuqton/hop/internal/apperr"
)

// Binary is the hop executable on disk, symlinks resolved.
type Binary struct {
	Path string
}

// Locate resolves the running executable so an update replaces the real file, not a symlink.
func Locate(executable func() (string, error)) (Binary, error) {
	p, err := executable()
	if err != nil {
		return Binary{}, apperr.New(apperr.ExitGeneral, "cannot find the hop executable: %v", err)
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		p = real
	}
	return Binary{Path: p}, nil
}

// Replace writes data next to the binary and renames it over the old one in one step.
func (b Binary) Replace(data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(b.Path), "."+filepath.Base(b.Path)+"-update-*")
	if err != nil {
		return b.writeError(err)
	}
	defer os.Remove(tmp.Name())
	_, werr := tmp.Write(data)
	if err := errors.Join(werr, tmp.Close(), os.Chmod(tmp.Name(), 0o755)); err != nil {
		return b.writeError(err)
	}
	if err := b.swap(tmp.Name()); err != nil {
		return b.writeError(err)
	}
	return nil
}

// StaleSuffix marks the previous binary that Windows could not delete while it was running.
const StaleSuffix = ".old"

// CleanStale deletes the binary a previous update or uninstall moved aside; errors are ignored.
func (b Binary) CleanStale() {
	_ = os.Remove(b.Path + StaleSuffix)
}

func (b Binary) writeError(err error) error {
	e := apperr.New(apperr.ExitGeneral, "cannot replace %s: %v", b.Path, err)
	if errors.Is(err, fs.ErrPermission) {
		return e.WithHint("%s", reinstallHint)
	}
	return e
}

// ItemKind says what a footprint entry is, for listings and JSON.
type ItemKind string

const (
	KindBinary     ItemKind = "binary"
	KindCompletion ItemKind = "completion"
	KindSkill      ItemKind = "skill"
	KindCache      ItemKind = "cache"
	KindConfig     ItemKind = "config"
)

// Item is one file or folder hop created on this machine.
type Item struct {
	Kind ItemKind `json:"kind"`
	Path string   `json:"path"`
	Dir  bool     `json:"-"`
}

// Footprint is everything hop may have written locally; it never touches the bucket.
type Footprint struct {
	Items []Item
}

// Existing keeps only the entries present on disk.
func (f Footprint) Existing() []Item {
	var out []Item
	for _, it := range f.Items {
		if _, err := os.Lstat(it.Path); err == nil {
			out = append(out, it)
		}
	}
	return out
}

// Remove deletes items (folders recursively), then empty parent folders named after hop.
func (f Footprint) Remove(items []Item) ([]Item, error) {
	var removed []Item
	var errs []error
	for _, it := range items {
		if err := f.removeItem(it); err != nil {
			errs = append(errs, apperr.New(apperr.ExitGeneral, "cannot remove %s: %v", it.Path, err))
			continue
		}
		removed = append(removed, it)
		f.pruneParent(it.Path)
	}
	return removed, errors.Join(errs...)
}

func (f Footprint) removeItem(it Item) error {
	switch {
	case it.Dir:
		return os.RemoveAll(it.Path)
	case it.Kind == KindBinary:
		return Binary{Path: it.Path}.Delete()
	}
	return os.Remove(it.Path)
}

// pruneParent removes the folder hop created around a file (e.g. ~/.config/hop) once it is empty.
func (f Footprint) pruneParent(p string) {
	dir := filepath.Dir(p)
	if filepath.Base(dir) == Defaults.Binary {
		_ = os.Remove(dir)
	}
}
