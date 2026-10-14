// Package cache keeps the local completion cache: device and shelf listings under ~/.cache/hop/<workspace_id>/.
package cache

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Policy holds cache timing and layout values from spec section 7.
var Policy = struct {
	StaleAfter  time.Duration
	TouchEvery  time.Duration
	AppDir      string
	DevicesFile string
	ShelvesDir  string
	TouchFile   string
	FileMode    fs.FileMode
	DirMode     fs.FileMode
}{
	StaleAfter:  30 * time.Second,
	TouchEvery:  5 * time.Minute,
	AppDir:      "hop",
	DevicesFile: "devices.json",
	ShelvesDir:  "shelves",
	TouchFile:   "last_touch.json",
	FileMode:    0o600,
	DirMode:     0o700,
}

// Device is one cached device entry.
type Device struct {
	Name     string    `json:"name"`
	LastSeen time.Time `json:"last_seen"`
}

// ShelfItem is one cached hot file entry.
type ShelfItem struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// Snapshot is a cached listing and the time it was fetched.
type Snapshot[T any] struct {
	FetchedAt time.Time `json:"fetched_at"`
	Items     []T       `json:"items"`
}

// Cache is the cache directory of one workspace.
type Cache struct {
	dir string
	Now func() time.Time
}

// Open returns the cache for a workspace under root (normally the user cache dir).
func Open(root, workspaceID string) *Cache {
	return &Cache{dir: filepath.Join(root, Policy.AppDir, workspaceID), Now: time.Now}
}

// DefaultRoot is $XDG_CACHE_HOME or ~/.cache.
func DefaultRoot() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return dir
	}
	return os.TempDir()
}

// Stale reports whether a snapshot should be refreshed in the background.
func (c *Cache) Stale(fetchedAt time.Time) bool {
	return c.Now().Sub(fetchedAt) > Policy.StaleAfter
}

// Devices reads the cached device list.
func (c *Cache) Devices() (Snapshot[Device], bool) {
	return jsonFile[Device]{c.path(Policy.DevicesFile)}.load()
}

// SaveDevices replaces the cached device list.
func (c *Cache) SaveDevices(items []Device) {
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	jsonFile[Device]{c.path(Policy.DevicesFile)}.save(Snapshot[Device]{FetchedAt: c.Now(), Items: items})
}

// Shelf reads a cached shelf listing.
func (c *Cache) Shelf(shelf string) (Snapshot[ShelfItem], bool) {
	return c.shelfFile(shelf).load()
}

// SaveShelf replaces a cached shelf listing after a full list call.
func (c *Cache) SaveShelf(shelf string, items []ShelfItem) {
	c.shelfFile(shelf).save(Snapshot[ShelfItem]{FetchedAt: c.Now(), Items: items})
}

// UpsertShelf updates entries locally without a list call, keeping the original fetch time.
func (c *Cache) UpsertShelf(shelf string, items ...ShelfItem) {
	f := c.shelfFile(shelf)
	snap, _ := f.load()
	byName := map[string]ShelfItem{}
	for _, it := range append(snap.Items, items...) {
		byName[it.Name] = it
	}
	snap.Items = snap.Items[:0]
	for _, it := range byName {
		snap.Items = append(snap.Items, it)
	}
	sort.Slice(snap.Items, func(i, j int) bool { return snap.Items[i].Name < snap.Items[j].Name })
	f.save(snap)
}

// RemoveFromShelf drops entries locally without a list call.
func (c *Cache) RemoveFromShelf(shelf string, names ...string) {
	f := c.shelfFile(shelf)
	snap, ok := f.load()
	if !ok {
		return
	}
	drop := map[string]bool{}
	for _, n := range names {
		drop[n] = true
	}
	kept := snap.Items[:0]
	for _, it := range snap.Items {
		if !drop[it.Name] {
			kept = append(kept, it)
		}
	}
	snap.Items = kept
	f.save(snap)
}

// DropShelf deletes a shelf's cache file, e.g. after a rename or purge.
func (c *Cache) DropShelf(shelf string) {
	_ = os.Remove(c.shelfFile(shelf).path)
}

// TouchDue reports whether last_seen should be written again, and records the touch when it is.
func (c *Cache) TouchDue() bool {
	f := jsonFile[struct{}]{c.path(Policy.TouchFile)}
	snap, ok := f.load()
	if ok && c.Now().Sub(snap.FetchedAt) < Policy.TouchEvery {
		return false
	}
	f.save(Snapshot[struct{}]{FetchedAt: c.Now()})
	return true
}

func (c *Cache) shelfFile(shelf string) jsonFile[ShelfItem] {
	return jsonFile[ShelfItem]{c.path(Policy.ShelvesDir, shelf+".json")}
}

func (c *Cache) path(parts ...string) string {
	return filepath.Join(append([]string{c.dir}, parts...)...)
}

// jsonFile is one atomically written cache file; corrupt files are deleted and read as empty.
type jsonFile[T any] struct {
	path string
}

func (f jsonFile[T]) load() (Snapshot[T], bool) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		return Snapshot[T]{}, false
	}
	var snap Snapshot[T]
	if json.Unmarshal(data, &snap) != nil {
		_ = os.Remove(f.path)
		return Snapshot[T]{}, false
	}
	return snap, true
}

func (f jsonFile[T]) save(snap Snapshot[T]) {
	data, err := json.Marshal(snap)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(f.path), Policy.DirMode); err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.path), ".tmp-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if errors.Join(werr, cerr) != nil || os.Rename(tmp.Name(), f.path) != nil {
		_ = os.Remove(tmp.Name())
	}
}
