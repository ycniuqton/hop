package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
)

// TestSharedStore checks @@: is one store every device reads and writes, and @@ is the shared shelf.
func TestSharedStore(t *testing.T) {
	a, b := newMemoryHarness(t).pair()
	assert.Equal(t, "ok: copied 1 file(s) → @@:docs/ (4 B)\n", a.ok("cp", a.file("guide.md", "read"), "@@:docs/"))
	assert.Contains(t, b.ok("ls", "-R", "@@:"), "docs/guide.md")

	b.ok("cp", "@@:docs/guide.md", b.path("guide.md"))
	got, err := os.ReadFile(b.path("guide.md"))
	require.NoError(t, err)
	assert.Equal(t, "read", string(got))
	b.ok("cp", b.file("notes.txt", "n"), "@@:docs/")
	assert.Contains(t, a.ok("ls", "@@:docs/"), "notes.txt")

	a.ok("cp", "@@:docs/notes.txt", "@web-1:kept/")
	assert.Contains(t, b.ok("ls", "@:kept/"), "notes.txt")
	assert.Equal(t, "ok: moved 1 file(s) → @@:old/notes.txt\n", a.ok("mv", "@@:docs/notes.txt", "@@:old/notes.txt"))
	assert.Equal(t, "ok: deleted 1 object(s) from @@:old/\n", a.ok("rm", "-r", "@@:old/"))
	a.ok("cp", a.file("mine.txt", "m"), "@:")
	assert.Equal(t, "ok: copied 1 file(s) → @laptop: (1 B)\n", a.ok("cp", "-f", a.path("mine.txt"), "@:"), "@: names this machine")
	assert.NotContains(t, b.ok("ls", "-R", "@@:"), "notes.txt")

	a.ok("push", a.file("x.txt", "x"), "@@")
	assert.Contains(t, b.ok("hot"), "x.txt", "@@ is the shared shelf")
	b.ok("keep", "x.txt", "@@", "@@:docs/")
	assert.Contains(t, a.ok("ls", "@@:docs/"), "x.txt")

	assert.Equal(t, "error: no such path: @@:nope", a.fails(apperr.ExitNotFound, "ls", "@@:nope"))
	status := a.ok("status")
	assert.Regexp(t, `hot\s+@@\s`, status)
	assert.Regexp(t, `store\s+@@:\s`, status)
	assert.Regexp(t, `store\s+@web-1:\s`, status)
	doc := jsonDoc(t, a.ok("cp", "@@:docs/guide.md", "@@:copy.md", "--json"))
	require.Equal(t, "cp", doc["action"])
}
