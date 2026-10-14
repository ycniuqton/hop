package workspace

import (
	"context"
	"errors"
	"slices"
	"sort"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/cache"
	"github.com/ycniuqton/hop/internal/naming"
	"github.com/ycniuqton/hop/internal/store"
	"github.com/ycniuqton/hop/internal/suggest"
)

// DeviceManager owns device records.
type DeviceManager struct {
	ws *Workspace
}

// ErrNameTaken is returned by Register when the device name already exists.
var ErrNameTaken = errors.New("device name taken")

// List reads every device record and refreshes the cache.
func (m *DeviceManager) List(ctx context.Context) ([]Device, error) {
	objs, err := m.ws.listAll(ctx, m.ws.Layout.DevicesPrefix())
	if err != nil {
		return nil, err
	}
	out := make([]Device, len(objs))
	err = Pool{Workers: Concurrency.Lookups}.Run(ctx, len(objs), func(ctx context.Context, i int) error {
		return m.ws.getJSON(ctx, objs[i].Key, &out[i])
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	m.saveCache(out)
	return out, nil
}

func (m *DeviceManager) saveCache(devs []Device) {
	if m.ws.Cache == nil {
		return
	}
	items := make([]cache.Device, len(devs))
	for i, d := range devs {
		items[i] = cache.Device{Name: d.Name, LastSeen: d.LastSeen}
	}
	m.ws.Cache.SaveDevices(items)
}

// Get reads one device record.
func (m *DeviceManager) Get(ctx context.Context, name string) (Device, error) {
	var d Device
	err := m.ws.getJSON(ctx, m.ws.Layout.DeviceKey(name), &d)
	if errors.Is(err, store.ErrNotFound) {
		return d, m.unknown(ctx, name)
	}
	return d, err
}

// Resolve checks that a device exists: cache first, S3 on a cache miss.
func (m *DeviceManager) Resolve(ctx context.Context, name string) error {
	if name == m.ws.Self || m.cachedNames().has(name) {
		return nil
	}
	_, err := m.ws.Storage.Head(ctx, m.ws.Layout.DeviceKey(name))
	if errors.Is(err, store.ErrNotFound) {
		return m.unknown(ctx, name)
	}
	return err
}

func (m *DeviceManager) unknown(ctx context.Context, name string) error {
	e := apperr.New(apperr.ExitNotFound, "unknown device %q", name).Wrap(store.ErrNotFound)
	devs, err := m.List(ctx)
	if err != nil {
		return e
	}
	names := make([]string, len(devs))
	for i, d := range devs {
		names[i] = d.Name
	}
	if s := suggest.DidYouMean.Closest(name, names); s != "" {
		return e.WithHint("did you mean %s?", s)
	}
	return e.WithHint("run 'hop devices' to see device names")
}

type nameSet []string

func (s nameSet) has(n string) bool { return slices.Contains(s, n) }

func (m *DeviceManager) cachedNames() nameSet {
	if m.ws.Cache == nil {
		return nil
	}
	snap, _ := m.ws.Cache.Devices()
	out := make(nameSet, len(snap.Items))
	for i, d := range snap.Items {
		out[i] = d.Name
	}
	return out
}

// Register writes a new device record; an existing name fails with ErrNameTaken.
func (m *DeviceManager) Register(ctx context.Context, d Device) error {
	if err := naming.Devices.Validate(d.Name); err != nil {
		return err
	}
	err := m.ws.putJSON(ctx, m.ws.Layout.DeviceKey(d.Name), d, true)
	if errors.Is(err, store.ErrPreconditionFailed) {
		return apperr.New(apperr.ExitConflict, "device name %q is already used", d.Name).
			WithHint("pick another with --name").Wrap(errors.Join(ErrNameTaken, err))
	}
	return err
}

// Touch updates this device's last_seen at most once per cache.Policy.TouchEvery.
func (m *DeviceManager) Touch(ctx context.Context) error {
	if m.ws.Cache == nil || m.ws.Self == "" || !m.ws.Cache.TouchDue() {
		return nil
	}
	d, err := m.Get(ctx, m.ws.Self)
	if err != nil {
		return err
	}
	d.LastSeen = m.ws.Now().UTC()
	return m.ws.putJSON(ctx, m.ws.Layout.DeviceKey(d.Name), d, false)
}
