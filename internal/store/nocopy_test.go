package store_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/store"
	"github.com/ycniuqton/hop/internal/store/storetest"
)

// noCopyObjectServer behaves like Bizfly: CopyObject answers 501, UploadPartCopy works.
// gofakes3 has no UploadPartCopy, so the shim serves it as a ranged read plus a normal UploadPart.
type noCopyObjectServer struct {
	next        http.Handler
	backend     *s3mem.Backend
	copyObjects atomic.Int32
}

func (s *noCopyObjectServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	source := r.Header.Get("x-amz-copy-source")
	switch {
	case source == "" || r.Method != http.MethodPut:
		s.next.ServeHTTP(w, r)
	case r.URL.Query().Get("uploadId") == "":
		s.copyObjects.Add(1)
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>NotImplemented</Code><Message></Message></Error>`))
	default:
		s.uploadPartCopy(w, r, source)
	}
}

func (s *noCopyObjectServer) uploadPartCopy(w http.ResponseWriter, r *http.Request, source string) {
	part, err := s.readRange(source, r.Header.Get("x-amz-copy-source-range"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	upload := r.Clone(r.Context())
	upload.Header.Del("x-amz-copy-source")
	upload.Header.Del("x-amz-copy-source-range")
	upload.Body, upload.ContentLength = io.NopCloser(bytes.NewReader(part)), int64(len(part))
	upload.Header.Set("Content-Length", strconv.Itoa(len(part)))
	rec := httptest.NewRecorder()
	s.next.ServeHTTP(rec, upload)
	if rec.Code != http.StatusOK {
		http.Error(w, rec.Body.String(), rec.Code)
		return
	}
	_, _ = fmt.Fprintf(w, `<CopyPartResult><ETag>%s</ETag><LastModified>2026-01-01T00:00:00.000Z</LastModified></CopyPartResult>`,
		rec.Header().Get("ETag"))
}

func (s *noCopyObjectServer) readRange(source, rng string) ([]byte, error) {
	decoded, err := url.PathUnescape(source)
	if err != nil {
		return nil, err
	}
	bucket, key, _ := strings.Cut(strings.TrimPrefix(decoded, "/"), "/")
	obj, err := s.backend.GetObject(bucket, key, nil)
	if err != nil {
		return nil, err
	}
	defer obj.Contents.Close()
	data, err := io.ReadAll(obj.Contents)
	if err != nil {
		return nil, err
	}
	var from, to int
	if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &from, &to); err != nil {
		return nil, err
	}
	return data[from : to+1], nil
}

func newNoCopyObjectS3(t *testing.T) (store.Storage, *noCopyObjectServer) {
	backend := s3mem.New()
	require.NoError(t, backend.CreateBucket("my-hop"))
	shim := &noCopyObjectServer{next: gofakes3.New(backend).Server(), backend: backend}
	srv := httptest.NewServer(shim)
	t.Cleanup(srv.Close)
	s, err := store.NewS3(context.Background(), store.Settings{
		Bucket: "my-hop", Region: "us-east-1", Endpoint: srv.URL, AccessKeyID: "AKIATEST", SecretAccessKey: "secret",
	})
	require.NoError(t, err)
	return s, shim
}

func TestS3ContractWithoutCopyObject(t *testing.T) {
	storetest.Contract(t, func(t *testing.T) store.Storage {
		if strings.HasSuffix(t.Name(), "/IfNoneMatch") {
			t.Skip("gofakes3 does not implement conditional writes")
		}
		s, _ := newNoCopyObjectS3(t)
		return s
	})
}

func TestCopyFallsBackToMultipartCopy(t *testing.T) {
	ctx := context.Background()
	s, shim := newNoCopyObjectS3(t)
	meta := map[string]string{"hop-from": "laptop", "hop-mode": "0640"}
	for name, body := range map[string]string{"tiny": "copy me", "empty": "", "key+with space": "x"} {
		require.NoError(t, s.Put(ctx, "src/"+name, strings.NewReader(body), int64(len(body)), store.PutOptions{Meta: meta}))
		require.NoError(t, s.Copy(ctx, "src/"+name, "dst/"+name), name)
		rc, info, err := s.Get(ctx, "dst/"+name)
		require.NoError(t, err, name)
		got, _ := io.ReadAll(rc)
		_ = rc.Close()
		assert.Equal(t, body, string(got), name)
		assert.Equal(t, meta, info.Meta, name)
	}
	assert.EqualValues(t, 1, shim.copyObjects.Load(), "CopyObject is tried once per client, then skipped")
}
