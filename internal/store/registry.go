package store

import (
	"context"
	"io"
	"sort"

	"github.com/ycniuqton/hop/internal/apperr"
)

// Backend names a storage implementation.
type Backend string

const (
	BackendS3     Backend = "s3"
	BackendMemory Backend = "memory"
)

// Settings is the connection information every backend receives.
type Settings struct {
	Backend         Backend
	Bucket          string
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Trace           io.Writer
}

// Factory opens a Storage for the given settings.
type Factory func(ctx context.Context, s Settings) (Storage, error)

// Manager selects a backend by configuration; business code opens storage only through it.
type Manager struct {
	factories map[Backend]Factory
	fallback  Backend
}

// NewManager returns a manager with the built-in backends registered.
func NewManager() *Manager {
	m := &Manager{factories: map[Backend]Factory{}, fallback: BackendS3}
	m.Register(BackendS3, NewS3)
	m.Register(BackendMemory, func(context.Context, Settings) (Storage, error) { return NewFake(), nil })
	return m
}

// UseDefault changes the backend used when Settings.Backend is empty (tests point it at memory).
func (m *Manager) UseDefault(b Backend) { m.fallback = b }

// Register adds or replaces a backend factory.
func (m *Manager) Register(b Backend, f Factory) { m.factories[b] = f }

// Open creates the storage named by s.Backend, or the default backend when unset.
func (m *Manager) Open(ctx context.Context, s Settings) (Storage, error) {
	b := s.Backend
	if b == "" {
		b = m.fallback
	}
	f, ok := m.factories[b]
	if !ok {
		return nil, apperr.New(apperr.ExitGeneral, "unknown storage backend %q (known: %v)", b, m.known())
	}
	return f(ctx, s)
}

func (m *Manager) known() []string {
	var out []string
	for b := range m.factories {
		out = append(out, string(b))
	}
	sort.Strings(out)
	return out
}
