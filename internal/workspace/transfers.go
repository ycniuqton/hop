package workspace

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/store"
)

// Transfers moves bytes between local files and object keys.
type Transfers struct {
	ws *Workspace
}

// TempSuffix marks partial downloads.
const TempSuffix = ".hop-tmp"

// LocalFile is a regular file ready to upload.
type LocalFile struct {
	Path string
	Size int64
	Mode fs.FileMode
}

// OpenLocal stats a local path and rejects non-regular files.
func (t *Transfers) OpenLocal(p string) (LocalFile, error) {
	info, err := os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return LocalFile{}, apperr.New(apperr.ExitNotFound, "no such file: %s", p)
	}
	if err != nil {
		return LocalFile{}, apperr.New(apperr.ExitGeneral, "cannot read %s: %v", p, err)
	}
	if info.IsDir() {
		return LocalFile{Path: p, Mode: info.Mode()}, nil
	}
	if !info.Mode().IsRegular() {
		return LocalFile{}, apperr.New(apperr.ExitUsage, "%s is not a regular file", p)
	}
	return LocalFile{Path: p, Size: info.Size(), Mode: info.Mode().Perm()}, nil
}

// Upload sends a local file to key with sender and mode metadata.
func (t *Transfers) Upload(ctx context.Context, f LocalFile, key string) error {
	fh, err := os.Open(f.Path)
	if err != nil {
		return apperr.New(apperr.ExitGeneral, "cannot open %s: %v", f.Path, err)
	}
	defer fh.Close()
	m := t.ws.meter(filepath.Base(f.Path), f.Size)
	defer m.Done()
	meta := map[string]string{Meta.From: t.ws.Self, Meta.Mode: fmt.Sprintf("%04o", f.Mode)}
	return t.ws.Storage.Put(ctx, key, m.Reader(fh), f.Size, store.PutOptions{Meta: meta})
}

// Download writes key to dest through a temp file and restores the stored mode.
func (t *Transfers) Download(ctx context.Context, key, dest string, force bool) (store.Object, error) {
	info, err := t.ws.Storage.Head(ctx, key)
	if err != nil {
		return info, err
	}
	if err := t.checkDest(dest, force); err != nil {
		return info, err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return info, apperr.New(apperr.ExitGeneral, "cannot create %s: %v", filepath.Dir(dest), err)
	}
	tmp := dest + TempSuffix
	if err := t.fetch(ctx, key, tmp, info.Size); err != nil {
		_ = os.Remove(tmp)
		return info, err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return info, apperr.New(apperr.ExitGeneral, "cannot write %s: %v", dest, err)
	}
	return info, t.restoreMode(dest, info.Meta[Meta.Mode])
}

func (t *Transfers) checkDest(dest string, force bool) error {
	if _, err := os.Lstat(dest); err == nil && !force {
		return apperr.New(apperr.ExitConflict, "exists: %s (use -f to overwrite)", dest)
	}
	return nil
}

func (t *Transfers) fetch(ctx context.Context, key, tmp string, size int64) error {
	fh, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return apperr.New(apperr.ExitGeneral, "cannot write %s: %v", tmp, err)
	}
	m := t.ws.meter(filepath.Base(key), size)
	_, err = t.ws.Storage.Download(ctx, key, m.WriterAt(fh))
	m.Done()
	return errors.Join(err, fh.Close())
}

func (t *Transfers) restoreMode(dest, mode string) error {
	perm, err := strconv.ParseUint(mode, 8, 32)
	if mode == "" || err != nil {
		perm = 0o644
	}
	return os.Chmod(dest, fs.FileMode(perm).Perm())
}
