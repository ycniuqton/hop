package cli

import (
	"bytes"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/store"
)

// conditionalWrites adds S3's If-None-Match: * semantics, which gofakes3 does not implement.
type conditionalWrites struct {
	next    http.Handler
	backend *s3mem.Backend
}

func (c conditionalWrites) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut && r.Header.Get("If-None-Match") == "*" {
		bucket, key, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if _, err := c.backend.HeadObject(bucket, key); err == nil {
			w.WriteHeader(http.StatusPreconditionFailed)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>PreconditionFailed</Code><Message>At least one of the pre-conditions you specified did not hold</Message></Error>`))
			return
		}
	}
	c.next.ServeHTTP(w, r)
}

func newS3Harness(t *testing.T) *harness {
	return newS3HarnessWith(t, func(h http.Handler) http.Handler { return h })
}

// newS3HarnessWith lets a test put its own middleware (fault injection) in front of the fake S3 server.
func newS3HarnessWith(t *testing.T, wrap func(http.Handler) http.Handler) *harness {
	backend := s3mem.New()
	require.NoError(t, backend.CreateBucket("my-hop"))
	srv := httptest.NewServer(wrap(conditionalWrites{next: gofakes3.New(backend).Server(), backend: backend}))
	t.Cleanup(srv.Close)
	return &harness{t: t, stores: store.NewManager(), initFlags: []string{"--endpoint", srv.URL, "--region", "us-east-1"}}
}

// TestIntegrationScriptOverS3 is the spec section 11 script, run through the real S3 client.
func TestIntegrationScriptOverS3(t *testing.T) {
	h := newS3Harness(t)
	a, b := h.pair()

	dup := h.machine("dup")
	code := strings.TrimSpace(a.ok("invite", "--code"))
	assert.Contains(t, dup.fails(apperr.ExitConflict, "join", code, "--name", "web-1"), "already used")

	src := a.file("x.bin", "hello over s3")
	a.ok("push", src, "@web-1")
	b.ok("pull", "x.bin", "@", b.path("x.bin"))
	got, _ := os.ReadFile(b.path("x.bin"))
	assert.Equal(t, "hello over s3", string(got))

	big := make([]byte, 50<<20)
	_, _ = rand.Read(big)
	b.file("folder/big.bin", string(big))
	b.file("folder/notes/n.txt", "n")
	b.ok("cp", "-r", b.path("folder"), "@:")
	assert.Equal(t, "folder/big.bin\nfolder/notes/n.txt\n", a.ok("ls", "-R", "@web-1:"))

	a.ok("cp", "@web-1:folder/big.bin", "@laptop:copy/")
	a.ok("cp", "@:copy/big.bin", a.path("big.bin"))
	back, _ := os.ReadFile(a.path("big.bin"))
	assert.True(t, bytes.Equal(big, back), "50 MB file round-trips byte for byte")

	a.ok("rm", "-r", "-y", "@web-1:folder/")
	assert.Empty(t, a.ok("ls", "@web-1:"))
}

func TestS3VerboseNeverPrintsSecret(t *testing.T) {
	a, _ := newS3Harness(t).pair()
	r := a.run("", "-v", "hot")
	require.Equal(t, apperr.ExitOK, r.code, r.err)
	assert.Contains(t, r.err, "s3: ListObjectsV2")
	assert.NotContains(t, r.err+r.out, "test-secret-value")
}
