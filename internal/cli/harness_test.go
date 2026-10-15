package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/store"
	"github.com/ycniuqton/hop/internal/ui"
)

// harness runs the real CLI in-process for several simulated machines sharing one storage backend.
type harness struct {
	t         *testing.T
	stores    *store.Manager
	initFlags []string
}

// machine is one simulated device; binary stands in for the hop executable, so update and
// uninstall never touch the test binary.
type machine struct {
	h       *harness
	name    string
	home    string
	vars    map[string]string
	spawner RefreshSpawner
	now     func() time.Time
	tty     bool
	version string
	binary  string
}

func (m *machine) executable() (string, error) { return m.binary, nil }

type result struct {
	out, err string
	code     apperr.ExitCode
}

type nopSpawner struct{}

func (nopSpawner) Spawn(RefreshTarget, ...string) {}

func newMemoryHarness(t *testing.T) *harness {
	fake := store.NewFake()
	m := store.NewManager()
	m.Register(store.BackendMemory, func(context.Context, store.Settings) (store.Storage, error) { return fake, nil })
	m.UseDefault(store.BackendMemory)
	return &harness{t: t, stores: m}
}

// machine creates a device with its own home; the path is resolved so Windows short names
// (C:\Users\RUNNER~1) match what hop reports after resolving its executable.
func (h *harness) machine(name string) *machine {
	home := h.t.TempDir()
	if real, err := filepath.EvalSymlinks(home); err == nil {
		home = real
	}
	return &machine{h: h, name: name, home: home, spawner: nopSpawner{}, now: time.Now, version: "test",
		binary: filepath.Join(home, ".local", "bin", "hop"), vars: map[string]string{
			EnvVars.Home: home, EnvVars.AWSKey: "AKIATEST", EnvVars.AWSSecret: "test-secret-value", EnvVars.AWSRegion: "us-east-1",
		}}
}

func (m *machine) run(stdin string, args ...string) result {
	return m.runCtx(context.Background(), stdin, args...)
}

func (m *machine) runCtx(ctx context.Context, stdin string, args ...string) result {
	var out, errb bytes.Buffer
	env := &Env{
		Console:  ui.NewConsole(strings.NewReader(stdin), &out, &errb),
		Getenv:   func(k string) string { return m.vars[k] },
		Hostname: func() (string, error) { return m.name + ".example.internal", nil },
		Stores:   m.h.stores,
		Spawner:  m.spawner,
		Version:  m.version,
		Now:      m.now,

		Executable: m.executable,
	}
	env.Console.InTTY = m.tty
	code := NewApp(env).Run(ctx, args)
	return result{out: out.String(), err: errb.String(), code: code}
}

func (m *machine) ok(args ...string) string {
	m.h.t.Helper()
	r := m.run("", args...)
	require.Equal(m.h.t, apperr.ExitOK, r.code, "hop %v\nstdout: %s\nstderr: %s", args, r.out, r.err)
	return r.out
}

func (m *machine) fails(code apperr.ExitCode, args ...string) string {
	m.h.t.Helper()
	r := m.run("", args...)
	require.Equal(m.h.t, code, r.code, "hop %v\nstdout: %s\nstderr: %s", args, r.out, r.err)
	return strings.TrimSpace(r.err)
}

func (m *machine) file(rel, body string) string {
	p := filepath.Join(m.home, rel)
	require.NoError(m.h.t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(m.h.t, os.WriteFile(p, []byte(body), 0o644))
	return p
}

func (m *machine) path(rel string) string { return filepath.Join(m.home, rel) }

// pair creates laptop (init) and web-1 (join).
func (h *harness) pair() (*machine, *machine) {
	a, b := h.machine("laptop"), h.machine("web-1")
	a.ok(append([]string{"init", "--bucket", "my-hop", "--name", "laptop"}, h.initFlags...)...)
	code := strings.TrimSpace(a.ok("invite", "--code"))
	b.ok("join", code, "--name", "web-1")
	return a, b
}
