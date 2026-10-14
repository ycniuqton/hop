package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
)

func TestSize(t *testing.T) {
	assert.Equal(t, "512 B", Size(512))
	assert.Equal(t, "1.5 KB", Size(1536))
	assert.Equal(t, "12 MB", Size(12<<20))
}

func TestAgo(t *testing.T) {
	now := time.Now()
	assert.Equal(t, "2m ago", Ago(now, now.Add(-2*time.Minute)))
	assert.Equal(t, "5s ago", Ago(now, now.Add(-5*time.Second)))
	assert.Equal(t, "3h ago", Ago(now, now.Add(-3*time.Hour)))
	assert.Equal(t, "3d ago", Ago(now, now.Add(-72*time.Hour)))
	assert.Equal(t, "never", Ago(now, time.Time{}))
}

func TestPromptsWithoutTTY(t *testing.T) {
	c := NewConsole(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	v, err := c.Ask("bucket", "def", "--bucket")
	require.NoError(t, err)
	assert.Equal(t, "def", v)
	_, err = c.Ask("bucket", "", "--bucket")
	assert.EqualError(t, err, "missing bucket — pass --bucket")
	_, err = c.Confirm("delete 12 objects?", false)
	assert.Equal(t, apperr.ExitUsage, apperr.CodeOf(err))
	assert.ErrorContains(t, err, "-y")
}

func TestPromptsWithTTY(t *testing.T) {
	c := NewConsole(strings.NewReader("my-hop\n\ny\n2\n"), &bytes.Buffer{}, &bytes.Buffer{})
	c.InTTY = true
	v, _ := c.Ask("bucket", "", "--bucket")
	assert.Equal(t, "my-hop", v)
	v, _ = c.Ask("region", "us-east-1", "--region")
	assert.Equal(t, "us-east-1", v)
	ok, _ := c.Confirm("go?", false)
	assert.True(t, ok)
	i, err := c.Pick("pick", []string{"a", "b"})
	require.NoError(t, err)
	assert.Equal(t, 1, i)
}

func TestTableAndQuiet(t *testing.T) {
	out := &bytes.Buffer{}
	c := NewConsole(strings.NewReader(""), out, &bytes.Buffer{})
	c.Table([]string{"NAME", "SIZE"}, [][]string{{"a.txt", "1 B"}})
	assert.Equal(t, "NAME   SIZE\na.txt  1 B\n", out.String())
	out.Reset()
	c.Quiet = true
	c.Printf("hi")
	c.Table([]string{"X"}, nil)
	assert.Empty(t, out.String())
}

func TestSuccessMarker(t *testing.T) {
	var out bytes.Buffer
	c := NewConsole(strings.NewReader(""), &out, &out)
	c.Color = true
	c.Successf("pushed %s", "a")
	c.Color = false
	c.Successf("pushed %s", "b")
	c.Quiet = true
	c.Successf("pushed %s", "c")
	assert.Equal(t, "✔ pushed a\nok: pushed b\n", out.String())
}
