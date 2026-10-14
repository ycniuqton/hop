// Package workspace holds hop's business logic: devices, hot shelves and the store, on top of store.Storage.
package workspace

import (
	"strings"

	"github.com/ycniuqton/hop/internal/naming"
)

// Layout maps workspace concepts to S3 keys (spec section 3).
type Layout struct {
	prefix string
}

// Paths are the fixed key segments of the bucket layout.
var Paths = struct {
	Meta, Devices, Hot, Store, WorkspaceFile, ProbeFile, JSONExt, Sep string
}{
	Meta: ".hop/", Devices: "devices/", Hot: "hot/", Store: "store/",
	WorkspaceFile: "workspace.json", ProbeFile: ".probe-", JSONExt: ".json", Sep: "/",
}

// NewLayout normalizes the optional prefix so it is empty or ends with "/".
func NewLayout(prefix string) Layout {
	p := strings.Trim(prefix, Paths.Sep)
	if p != "" {
		p += Paths.Sep
	}
	return Layout{prefix: p}
}

func (l Layout) WorkspaceKey() string         { return l.prefix + Paths.Meta + Paths.WorkspaceFile }
func (l Layout) DevicesPrefix() string        { return l.prefix + Paths.Meta + Paths.Devices }
func (l Layout) DeviceKey(name string) string { return l.DevicesPrefix() + name + Paths.JSONExt }
func (l Layout) ProbeKey(token string) string {
	return l.prefix + Paths.Meta + Paths.ProbeFile + token
}
func (l Layout) HotRoot() string                  { return l.prefix + Paths.Hot }
func (l Layout) ShelfPrefix(shelf string) string  { return l.HotRoot() + shelf + Paths.Sep }
func (l Layout) HotKey(shelf, name string) string { return l.ShelfPrefix(shelf) + name }
func (l Layout) StoreRoot() string                { return l.prefix + Paths.Store }
func (l Layout) StorePrefix(device string) string { return l.StoreRoot() + device + Paths.Sep }

// StoreKey maps @device:path to its key; an empty path is the device root.
func (l Layout) StoreKey(device, p string) string { return l.StorePrefix(device) + p }

// DirPrefix is the listing prefix for a store folder.
func (l Layout) DirPrefix(device, p string) string {
	if p == "" {
		return l.StorePrefix(device)
	}
	return l.StoreKey(device, p) + Paths.Sep
}

// DeviceFromKey extracts the device name from a devices/<name>.json key.
func (l Layout) DeviceFromKey(key string) string {
	return strings.TrimSuffix(strings.TrimPrefix(key, l.DevicesPrefix()), Paths.JSONExt)
}

// ShelfOf returns the shelf segment of a hot key, or "" when the key is not under hot/.
func (l Layout) ShelfOf(key string) string {
	rest, ok := strings.CutPrefix(key, l.HotRoot())
	if !ok {
		return ""
	}
	shelf, _, _ := strings.Cut(rest, Paths.Sep)
	return shelf
}

// DeviceOfStoreKey returns the device segment of a store key.
func (l Layout) DeviceOfStoreKey(key string) string {
	rest, _ := strings.CutPrefix(key, l.StoreRoot())
	dev, _, _ := strings.Cut(rest, Paths.Sep)
	return dev
}

// Shelf names a hot shelf: the shared one or a device's.
type Shelf string

// SharedShelf is the global hot list.
const SharedShelf Shelf = naming.SharedShelf

// IsShared reports whether this is the shared shelf.
func (s Shelf) IsShared() bool { return s == SharedShelf }

// Label is the human description used in messages.
func (s Shelf) Label() string {
	if s.IsShared() {
		return "shared shelf"
	}
	return "@" + string(s) + "'s shelf"
}

// RemoteRef is the @device:path spelling of a store location.
func RemoteRef(device, p string) string { return "@" + device + ":" + p }

// String is the normalized prefix ("" or "name/").
func (l Layout) String() string { return l.prefix }
