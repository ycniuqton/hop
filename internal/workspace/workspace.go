package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/cache"
	"github.com/ycniuqton/hop/internal/store"
)

// Workspace is one opened workspace as seen from one device.
type Workspace struct {
	Storage store.Storage
	Layout  Layout
	Self    string
	Cache   *cache.Cache
	Now     func() time.Time
	Version string
	Meter   MeterFactory
}

// Meter reports transfer progress; ui.Progress satisfies it.
type Meter interface {
	Reader(r io.Reader) io.Reader
	WriterAt(w io.WriterAt) io.WriterAt
	Done()
}

// MeterFactory starts a meter for one transfer.
type MeterFactory func(label string, total int64) Meter

// Services bundles the managers business code talks to.
type Services struct {
	Devices *DeviceManager
	Shelves *ShelfManager
	Store   *StoreManager
	Status  *StatusManager
}

// NewServices wires every manager for a workspace.
func NewServices(ws *Workspace) *Services {
	devices := &DeviceManager{ws: ws}
	transfers := &Transfers{ws: ws}
	return &Services{
		Devices: devices,
		Shelves: &ShelfManager{ws: ws, devices: devices, transfers: transfers},
		Store:   &StoreManager{ws: ws, devices: devices, transfers: transfers},
		Status:  &StatusManager{ws: ws},
	}
}

func (w *Workspace) meter(label string, total int64) Meter {
	if w.Meter == nil {
		return silentMeter{}
	}
	return w.Meter(label, total)
}

func (w *Workspace) getJSON(ctx context.Context, key string, v any) error {
	rc, _, err := w.Storage.Get(ctx, key)
	if err != nil {
		return err
	}
	defer rc.Close()
	if err := json.NewDecoder(rc).Decode(v); err != nil {
		return apperr.New(apperr.ExitGeneral, "corrupt workspace object %s: %v", key, err)
	}
	return nil
}

func (w *Workspace) putJSON(ctx context.Context, key string, v any, ifNoneMatch bool) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return w.Storage.Put(ctx, key, bytes.NewReader(data), int64(len(data)), store.PutOptions{IfNoneMatch: ifNoneMatch})
}

// listAll returns every object under a prefix.
func (w *Workspace) listAll(ctx context.Context, prefix string) ([]store.Object, error) {
	objs, _, err := w.Storage.List(ctx, prefix, "", store.Limits.NoLimit)
	return objs, err
}

type silentMeter struct{}

func (silentMeter) Reader(r io.Reader) io.Reader       { return r }
func (silentMeter) WriterAt(w io.WriterAt) io.WriterAt { return w }
func (silentMeter) Done()                              {}
