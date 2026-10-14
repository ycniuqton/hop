package workspace

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/store"
)

// interruptedStorage writes part of the object, then fails the way a Ctrl-C mid-download does.
type interruptedStorage struct {
	store.Storage
}

func (s interruptedStorage) Download(_ context.Context, _ string, w io.WriterAt) (int64, error) {
	n, _ := w.WriteAt([]byte("partial"), 0)
	return int64(n), context.Canceled
}

func TestInterruptedDownloadLeavesNoTempFile(t *testing.T) {
	fake := store.NewFake()
	require.NoError(t, fake.Put(ctx, "k", strings.NewReader("full body"), 9, store.PutOptions{}))
	ws := &Workspace{Storage: interruptedStorage{fake}, Layout: NewLayout(""), Now: time.Now}
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.bin")

	_, err := (&Transfers{ws: ws}).Download(ctx, "k", dest, false)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, apperr.ExitInterrupted, apperr.CodeOf(err))
	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries, "neither the destination nor %s may remain", TempSuffix)
}
