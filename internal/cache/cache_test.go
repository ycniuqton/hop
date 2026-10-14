package cache

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCache(t *testing.T) (*Cache, *time.Time) {
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	c := Open(t.TempDir(), "ws-1")
	c.Now = func() time.Time { return now }
	return c, &now
}

func TestDevicesRoundTripAndStaleness(t *testing.T) {
	c, now := newCache(t)
	_, ok := c.Devices()
	assert.False(t, ok)

	c.SaveDevices([]Device{{Name: "web-1"}, {Name: "laptop"}})
	snap, ok := c.Devices()
	require.True(t, ok)
	assert.Equal(t, "laptop", snap.Items[0].Name)
	assert.False(t, c.Stale(snap.FetchedAt))

	*now = now.Add(31 * time.Second)
	assert.True(t, c.Stale(snap.FetchedAt))
}

func TestShelfWriteThrough(t *testing.T) {
	c, _ := newCache(t)
	c.SaveShelf("_shared", []ShelfItem{{Name: "a", Size: 1}})
	c.UpsertShelf("_shared", ShelfItem{Name: "b", Size: 2}, ShelfItem{Name: "a", Size: 9})
	snap, _ := c.Shelf("_shared")
	assert.Equal(t, []ShelfItem{{Name: "a", Size: 9}, {Name: "b", Size: 2}}, snap.Items)

	c.RemoveFromShelf("_shared", "a")
	snap, _ = c.Shelf("_shared")
	assert.Equal(t, []ShelfItem{{Name: "b", Size: 2}}, snap.Items)

	c.DropShelf("_shared")
	_, ok := c.Shelf("_shared")
	assert.False(t, ok)
}

func TestCorruptFileIsDeleted(t *testing.T) {
	c, _ := newCache(t)
	c.SaveDevices(nil)
	path := c.path(Policy.DevicesFile)
	require.NoError(t, os.WriteFile(path, []byte("{oops"), 0o600))
	_, ok := c.Devices()
	assert.False(t, ok)
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestTouchDueThrottles(t *testing.T) {
	c, now := newCache(t)
	assert.True(t, c.TouchDue())
	assert.False(t, c.TouchDue())
	*now = now.Add(5*time.Minute + time.Second)
	assert.True(t, c.TouchDue())
}
