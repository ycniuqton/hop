package selfinstall

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"path"

	"github.com/ycniuqton/hop/internal/apperr"
)

// ArchiveFormat is how a release archive is packed (it matches .goreleaser.yaml).
type ArchiveFormat interface {
	Ext() string
	Extract(archive []byte, executable string) ([]byte, error)
}

// Packaging is the archive format and executable name used for one OS.
type Packaging struct {
	Format     ArchiveFormat
	Executable string
}

var packagings = map[string]Packaging{"windows": {Format: zipFormat{}, Executable: Defaults.Binary + ".exe"}}

var defaultPackaging = Packaging{Format: tarGzFormat{}, Executable: Defaults.Binary}

// PackagingFor returns the packaging of an OS; adding a platform means adding a registry entry.
func PackagingFor(goos string) Packaging {
	if p, ok := packagings[goos]; ok {
		return p
	}
	return defaultPackaging
}

type tarGzFormat struct{}

func (tarGzFormat) Ext() string { return ".tar.gz" }

func (tarGzFormat) Extract(archive []byte, executable string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, apperr.New(apperr.ExitGeneral, "release archive is not gzip: %v", err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, apperr.New(apperr.ExitGeneral, "release archive has no %q", executable)
		}
		if err != nil {
			return nil, apperr.New(apperr.ExitGeneral, "release archive is corrupt: %v", err)
		}
		if h.Typeflag == tar.TypeReg && path.Base(h.Name) == executable {
			return io.ReadAll(io.LimitReader(tr, Defaults.MaxDownload))
		}
	}
}

type zipFormat struct{}

func (zipFormat) Ext() string { return ".zip" }

func (zipFormat) Extract(archive []byte, executable string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, apperr.New(apperr.ExitGeneral, "release archive is not a zip: %v", err)
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || path.Base(f.Name) != executable {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, apperr.New(apperr.ExitGeneral, "release archive is corrupt: %v", err)
		}
		defer rc.Close()
		return io.ReadAll(io.LimitReader(rc, Defaults.MaxDownload))
	}
	return nil, apperr.New(apperr.ExitGeneral, "release archive has no %q", executable)
}
