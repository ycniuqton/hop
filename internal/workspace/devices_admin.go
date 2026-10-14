package workspace

import (
	"context"
	"errors"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/naming"
	"github.com/ycniuqton/hop/internal/store"
)

// RenameResult reports what moved during a rename.
type RenameResult struct {
	HotObjects   int
	StoreObjects int
	Self         bool
}

// Rename copies the device record and moves its shelf and store folder to the new name.
func (m *DeviceManager) Rename(ctx context.Context, oldName, newName string) (RenameResult, error) {
	if err := naming.Devices.Validate(newName); err != nil {
		return RenameResult{}, err
	}
	d, err := m.Get(ctx, oldName)
	if err != nil {
		return RenameResult{}, err
	}
	d.Name = newName
	if err := m.Register(ctx, d); err != nil {
		return RenameResult{}, err
	}
	res, err := m.moveData(ctx, oldName, newName)
	if err != nil {
		return res, err
	}
	if err := m.ws.Storage.Delete(ctx, m.ws.Layout.DeviceKey(oldName)); err != nil {
		return res, err
	}
	res.Self = oldName == m.ws.Self
	m.afterRename(ctx, oldName, newName, res.Self)
	return res, nil
}

func (m *DeviceManager) moveData(ctx context.Context, oldName, newName string) (RenameResult, error) {
	l, bulk := m.ws.Layout, Bulk{ws: m.ws}
	hot, err := bulk.MovePrefix(ctx, l.ShelfPrefix(oldName), l.ShelfPrefix(newName))
	if err != nil {
		return RenameResult{}, err
	}
	st, err := bulk.MovePrefix(ctx, l.StorePrefix(oldName), l.StorePrefix(newName))
	return RenameResult{HotObjects: hot, StoreObjects: st}, err
}

func (m *DeviceManager) afterRename(ctx context.Context, oldName, newName string, self bool) {
	if self {
		m.ws.Self = newName
	}
	if m.ws.Cache != nil {
		m.ws.Cache.DropShelf(oldName)
		m.ws.Cache.DropShelf(newName)
	}
	_, _ = m.List(ctx)
}

// RemoveResult reports what a removal deleted.
type RemoveResult struct {
	Purged int
}

// Remove deletes a device record, and with purge its shelf and store folder.
func (m *DeviceManager) Remove(ctx context.Context, name string, purge bool) (RemoveResult, error) {
	if name == m.ws.Self {
		return RemoveResult{}, apperr.New(apperr.ExitUsage, "cannot remove this device (%s) from itself", name).
			WithHint("run 'hop remove %s' on another device", name)
	}
	if _, err := m.ws.Storage.Head(ctx, m.ws.Layout.DeviceKey(name)); errors.Is(err, store.ErrNotFound) {
		return RemoveResult{}, m.unknown(ctx, name)
	} else if err != nil {
		return RemoveResult{}, err
	}
	var res RemoveResult
	if purge {
		n, err := m.purge(ctx, name)
		if err != nil {
			return res, err
		}
		res.Purged = n
	}
	if err := m.ws.Storage.Delete(ctx, m.ws.Layout.DeviceKey(name)); err != nil {
		return res, err
	}
	if m.ws.Cache != nil {
		m.ws.Cache.DropShelf(name)
	}
	_, _ = m.List(ctx)
	return res, nil
}

func (m *DeviceManager) purge(ctx context.Context, name string) (int, error) {
	bulk := Bulk{ws: m.ws}
	hot, err := bulk.DeletePrefix(ctx, m.ws.Layout.ShelfPrefix(name))
	if err != nil {
		return hot, err
	}
	st, err := bulk.DeletePrefix(ctx, m.ws.Layout.StorePrefix(name))
	return hot + st, err
}
