// Package storetest is a behavior contract every Storage implementation must pass (fake in unit tests, MinIO in integration).
package storetest

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/store"
)

// Contract runs every storage behavior hop relies on against a fresh store from newStore.
func Contract(t *testing.T, newStore func(t *testing.T) store.Storage) {
	suite := contractSuite{newStore: newStore}
	t.Run("PutGetHead", suite.putGetHead)
	t.Run("IfNoneMatch", suite.ifNoneMatch)
	t.Run("NotFound", suite.notFound)
	t.Run("Copy", suite.copy)
	t.Run("Delete", suite.delete)
	t.Run("ListDelimiter", suite.listDelimiter)
	t.Run("ListLimit", suite.listLimit)
	t.Run("Download", suite.download)
}

type contractSuite struct {
	newStore func(t *testing.T) store.Storage
}

func (c contractSuite) put(t *testing.T, s store.Storage, key, body string, opt store.PutOptions) error {
	return s.Put(context.Background(), key, strings.NewReader(body), int64(len(body)), opt)
}

func (c contractSuite) read(t *testing.T, s store.Storage, key string) (string, store.Object) {
	rc, info, err := s.Get(context.Background(), key)
	require.NoError(t, err)
	defer rc.Close()
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	return string(data), info
}

func (c contractSuite) putGetHead(t *testing.T) {
	s := c.newStore(t)
	meta := map[string]string{"hop-from": "web-1", "hop-mode": "0644"}
	require.NoError(t, c.put(t, s, "hot/_shared/a.txt", "hello", store.PutOptions{Meta: meta}))
	body, info := c.read(t, s, "hot/_shared/a.txt")
	assert.Equal(t, "hello", body)
	assert.EqualValues(t, 5, info.Size)
	assert.Equal(t, meta, info.Meta)

	head, err := s.Head(context.Background(), "hot/_shared/a.txt")
	require.NoError(t, err)
	assert.EqualValues(t, 5, head.Size)
	assert.Equal(t, "web-1", head.Meta["hop-from"])

	require.NoError(t, c.put(t, s, "hot/_shared/a.txt", "v2", store.PutOptions{}))
	body, _ = c.read(t, s, "hot/_shared/a.txt")
	assert.Equal(t, "v2", body)
}

func (c contractSuite) ifNoneMatch(t *testing.T) {
	s := c.newStore(t)
	opt := store.PutOptions{IfNoneMatch: true}
	require.NoError(t, c.put(t, s, ".hop/devices/web-1.json", "{}", opt))
	err := c.put(t, s, ".hop/devices/web-1.json", "{}", opt)
	assert.ErrorIs(t, err, store.ErrPreconditionFailed)
}

func (c contractSuite) notFound(t *testing.T) {
	s := c.newStore(t)
	_, err := s.Head(context.Background(), "missing")
	assert.ErrorIs(t, err, store.ErrNotFound)
	_, _, err = s.Get(context.Background(), "missing")
	assert.ErrorIs(t, err, store.ErrNotFound)
	assert.ErrorIs(t, s.Copy(context.Background(), "missing", "x"), store.ErrNotFound)
}

func (c contractSuite) copy(t *testing.T) {
	s := c.newStore(t)
	require.NoError(t, c.put(t, s, "store/web-1/a b+c.sql", "data", store.PutOptions{Meta: map[string]string{"hop-mode": "0600"}}))
	require.NoError(t, s.Copy(context.Background(), "store/web-1/a b+c.sql", "store/db-1/restore/a.sql"))
	body, info := c.read(t, s, "store/db-1/restore/a.sql")
	assert.Equal(t, "data", body)
	assert.Equal(t, "0600", info.Meta["hop-mode"])
}

func (c contractSuite) delete(t *testing.T) {
	s := c.newStore(t)
	require.NoError(t, c.put(t, s, "a", "1", store.PutOptions{}))
	require.NoError(t, c.put(t, s, "b", "2", store.PutOptions{}))
	require.NoError(t, s.Delete(context.Background(), "a", "b", "never-existed"))
	_, err := s.Head(context.Background(), "a")
	assert.ErrorIs(t, err, store.ErrNotFound)
	require.NoError(t, s.Delete(context.Background()))
}

func (c contractSuite) seedTree(t *testing.T, s store.Storage) {
	for _, k := range []string{"store/w/notes.txt", "store/w/backups/1.sql", "store/w/backups/2.sql", "store/w/logs/x/y.log", "store/v/z"} {
		require.NoError(t, c.put(t, s, k, "x", store.PutOptions{}))
	}
}

func (c contractSuite) listDelimiter(t *testing.T) {
	s := c.newStore(t)
	c.seedTree(t, s)
	objs, dirs, err := s.List(context.Background(), "store/w/", "/", store.Limits.NoLimit)
	require.NoError(t, err)
	assert.Equal(t, []string{"store/w/backups/", "store/w/logs/"}, dirs)
	require.Len(t, objs, 1)
	assert.Equal(t, "store/w/notes.txt", objs[0].Key)

	objs, dirs, err = s.List(context.Background(), "store/w/", "", store.Limits.NoLimit)
	require.NoError(t, err)
	assert.Empty(t, dirs)
	assert.Len(t, objs, 4)
}

func (c contractSuite) listLimit(t *testing.T) {
	s := c.newStore(t)
	c.seedTree(t, s)
	objs, dirs, err := s.List(context.Background(), "store/", "", 2)
	require.NoError(t, err)
	assert.Len(t, objs, 2)
	assert.Empty(t, dirs)
}

func (c contractSuite) download(t *testing.T) {
	s := c.newStore(t)
	require.NoError(t, c.put(t, s, "d/x.bin", "payload", store.PutOptions{}))
	f, err := os.CreateTemp(t.TempDir(), "dl")
	require.NoError(t, err)
	defer f.Close()
	n, err := s.Download(context.Background(), "d/x.bin", f)
	require.NoError(t, err)
	assert.EqualValues(t, 7, n)
	data, _ := os.ReadFile(f.Name())
	assert.Equal(t, "payload", string(data))
	_, err = s.Download(context.Background(), "d/missing", f)
	assert.ErrorIs(t, err, store.ErrNotFound)
	assert.NoError(t, s.CheckBucket(context.Background()))
}
