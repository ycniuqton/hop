package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/store"
)

// TestDidYouMeanEverywhere checks every command that takes a device or a hot name suggests the near miss.
func TestDidYouMeanEverywhere(t *testing.T) {
	a, _ := completionWorld(t)
	device := `error: unknown device "web1" — did you mean web-1?`
	for _, args := range [][]string{
		{"push", a.path("dump.sql"), "@web1"},
		{"pull", "site.tar", "@web1"},
		{"hot", "@web1"},
		{"hot", "rm", "site.tar", "@web1"},
		{"hot", "clear", "@web1", "-y"},
		{"cp", a.path("dump.sql"), "@web1:x"},
		{"ls", "@web1:"},
		{"rm", "@web1:notes.txt"},
		{"rename", "web1", "web-2"},
		{"remove", "web1", "-y"},
		{"keep", "dump.sql", "@web1:x"},
	} {
		assert.Equal(t, device, a.fails(apperr.ExitNotFound, args...), "%v", args)
	}
	shared := `error: "dmp.sql" not on shared shelf — did you mean dump.sql?`
	for _, args := range [][]string{
		{"pull", "dmp.sql", a.path("out")},
		{"hot", "rm", "dmp.sql"},
		{"keep", "dmp.sql", "@:x"},
	} {
		assert.Equal(t, shared, a.fails(apperr.ExitNotFound, args...), "%v", args)
	}
	assert.Equal(t, `error: "site.ta" not on @web-1's shelf — did you mean site.tar?`,
		a.fails(apperr.ExitNotFound, "pull", "site.ta", "@web-1", a.path("out")))
}

// denyPuts answers every PutObject with S3's AccessDenied once armed.
type denyPuts struct {
	next  http.Handler
	armed *atomic.Bool
}

func (d denyPuts) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if d.armed.Load() && r.Method == http.MethodPut {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`))
		return
	}
	d.next.ServeHTTP(w, r)
}

// TestSpecErrorExamples reproduces each spec section 10 error line with its exit code.
func TestSpecErrorExamples(t *testing.T) {
	armed := &atomic.Bool{}
	h := newS3HarnessWith(t, func(next http.Handler) http.Handler { return denyPuts{next: next, armed: armed} })
	a, b := h.pair()
	src := a.file("dump.sql", "x")
	a.ok("push", src, "@")
	a.ok("cp", src, "@web-1:backups/db.sql")

	examples := []struct {
		m    *machine
		code apperr.ExitCode
		want string
		args []string
	}{
		{h.machine("fresh"), apperr.ExitGeneral, "error: not configured — run 'hop init' or 'hop join <code>'", []string{"hot"}},
		{a, apperr.ExitNotFound, `error: "dump.sql" not on shared shelf — did you mean: hop pull dump.sql @`, []string{"pull", "dump.sql", a.path("o")}},
		{a, apperr.ExitNotFound, `error: unknown device "web1" — did you mean web-1?`, []string{"ls", "@web1:"}},
		{b, apperr.ExitConflict, "error: exists: @web-1:backups/db.sql (use -f to overwrite)", []string{"cp", b.file("db.sql", "y"), "@:backups/"}},
	}
	for _, e := range examples {
		assert.Equal(t, e.want, e.m.fails(e.code, e.args...), "%v", e.args)
	}

	a.ok("push", src)
	assert.Equal(t, `error: "dmp.sql" not on shared shelf — did you mean dump.sql?`, b.fails(apperr.ExitNotFound, "pull", "dmp.sql", b.path("o")))

	armed.Store(true)
	assert.Equal(t, "error: S3 access denied on PutObject my-hop/hot/_shared/x — check the IAM policy ('hop init --print-policy')",
		a.fails(apperr.ExitAuth, "push", a.file("x", "x")))
}

func TestNetworkErrorExitCode(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	m := newMemoryHarness(t).machine("x")
	h := &harness{t: t, stores: store.NewManager()}
	m.h = h
	err := m.fails(apperr.ExitNetwork, "init", "--bucket", "my-hop", "--name", "x", "--endpoint", url, "--region", "us-east-1")
	assert.Contains(t, err, "network error")
	assert.NotContains(t, err, "test-secret-value")
}

func TestInterruptExitCode(t *testing.T) {
	a, _ := newMemoryHarness(t).pair()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := a.runCtx(ctx, "", "push", a.file("x", "x"))
	assert.Equal(t, apperr.ExitInterrupted, r.code)
	assert.Empty(t, r.err, "Ctrl-C exits quietly")
}
