// Package selfinstalltest serves a fake GitHub release channel for update tests.
package selfinstalltest

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/ycniuqton/hop/internal/selfinstall"
)

// Server publishes one release tag whose archive holds Binary as the hop executable.
type Server struct {
	URL       string
	Tag       string
	Binary    []byte
	BadSum    bool
	Downloads int
}

// Start serves /<repo>/releases/latest (302 to the tag) and the tag's assets.
func Start(t *testing.T, repo, tag string, binary []byte) *Server {
	s := &Server{Tag: tag, Binary: binary}
	archive := s.archive(t)
	asset := selfinstall.NewReleases("", "").Asset(runtime.GOOS, runtime.GOARCH)
	mux := http.NewServeMux()
	mux.HandleFunc("/"+repo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/"+repo+"/releases/tag/"+s.Tag, http.StatusFound)
	})
	mux.HandleFunc("/"+repo+"/releases/download/"+tag+"/"+asset, func(w http.ResponseWriter, _ *http.Request) {
		s.Downloads++
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/"+repo+"/releases/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		sum := sha256.Sum256(archive)
		if s.BadSum {
			sum[0] ^= 0xff
		}
		_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + asset + "\n"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s.URL = srv.URL
	return s
}

// archive packs Binary the way the release for this OS is packed (zip + hop.exe on Windows).
func (s *Server) archive(t *testing.T) []byte {
	pk := selfinstall.PackagingFor(runtime.GOOS)
	files := map[string][]byte{"README.md": []byte("readme"), pk.Executable: s.Binary}
	if pk.Format.Ext() == ".zip" {
		return s.zip(t, files)
	}
	return s.tarGz(t, files)
}

func (s *Server) tarGz(t *testing.T, files map[string][]byte) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		_, _ = tw.Write(body)
	}
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

func (s *Server) zip(t *testing.T, files map[string][]byte) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(body)
	}
	_ = zw.Close()
	return buf.Bytes()
}
