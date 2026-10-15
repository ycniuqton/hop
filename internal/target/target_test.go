package target

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var p = Parser{Self: "laptop"}

func TestParseRules(t *testing.T) {
	cases := map[string]Target{
		"dump.sql":         {Kind: Local, Path: "dump.sql"},
		"./@x":             {Kind: Local, Path: "./@x"},
		"/abs/dir/":        {Kind: Local, Path: "/abs/dir/", Dir: true},
		"@web-1":           {Kind: Device, Device: "web-1"},
		"@":                {Kind: Device, Device: "laptop"},
		"@me":              {Kind: Device, Device: "laptop"},
		"@web-1:":          {Kind: Remote, Device: "web-1", Dir: true},
		"@:":               {Kind: Remote, Device: "laptop", Dir: true},
		"@me:notes.txt":    {Kind: Remote, Device: "laptop", Path: "notes.txt"},
		"@web-1:backups/":  {Kind: Remote, Device: "web-1", Path: "backups", Dir: true},
		"@web-1:a:b.txt":   {Kind: Remote, Device: "web-1", Path: "a:b.txt"},
		"@web-1:/x//y/./z": {Kind: Remote, Device: "web-1", Path: "x/y/z"},
		"@web-1:a/../b":    {Kind: Remote, Device: "web-1", Path: "b"},
		"@web-1:.":         {Kind: Remote, Device: "web-1", Dir: true},
		"@web-1:///":       {Kind: Remote, Device: "web-1", Dir: true},
	}
	for in, want := range cases {
		got, err := p.Parse(in)
		require.NoError(t, err, in)
		want.Raw = in
		assert.Equal(t, want, got, in)
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"@web-1:..":      "escapes",
		"@web-1:../etc":  "escapes",
		"@web-1:a/../..": "escapes",
		"@Web":           "invalid device name",
		"@_shared":       "reserved",
		"@bad_name:x":    "invalid device name",
	}
	for in, want := range cases {
		_, err := p.Parse(in)
		assert.ErrorContains(t, err, want, in)
	}
}

func TestForHotDeviceInAnyPosition(t *testing.T) {
	for _, args := range [][]string{{"dump.sql", "@web-1"}, {"@web-1", "dump.sql"}} {
		l, err := p.ParseAll(args)
		require.NoError(t, err)
		h, err := l.ForHot(false)
		require.NoError(t, err)
		require.NotNil(t, h.Shelf)
		assert.Equal(t, "web-1", h.Shelf.Device)
		assert.Equal(t, "dump.sql", h.Others[0].Path)
	}
}

func TestForHotRules(t *testing.T) {
	l, _ := p.ParseAll([]string{"a", "@web-1", "@db-1"})
	_, err := l.ForHot(false)
	assert.ErrorContains(t, err, "only one @device")

	l, _ = p.ParseAll([]string{"@web-1:x"})
	_, err = l.ForHot(false)
	assert.ErrorContains(t, err, "store path")
	_, err = l.ForHot(true)
	assert.NoError(t, err)

	l, _ = p.ParseAll([]string{"a"})
	h, err := l.ForHot(false)
	require.NoError(t, err)
	assert.Nil(t, h.Shelf)
}

func TestForStoreRejectsBareDevice(t *testing.T) {
	l, _ := p.ParseAll([]string{"db.sql", "@web-1"})
	_, err := l.ForStore()
	assert.ErrorContains(t, err, "did you mean @web-1: (with a colon)?")
}

func TestDevicesDistinct(t *testing.T) {
	l, _ := p.ParseAll([]string{"x", "@web-1:a", "@web-1", "@", "@db-1:"})
	assert.Equal(t, []string{"web-1", "laptop", "db-1"}, l.Devices())
}

func TestLocalFolderWithPlatformSeparator(t *testing.T) {
	tg, err := Parser{Self: "me"}.Parse("backups" + string(filepath.Separator))
	require.NoError(t, err)
	assert.True(t, tg.Dir)
	tg, err = Parser{Self: "me"}.Parse("backups/")
	require.NoError(t, err)
	assert.True(t, tg.Dir)
}
