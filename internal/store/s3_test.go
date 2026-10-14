package store_test

import (
	"bytes"
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/store"
	"github.com/ycniuqton/hop/internal/store/storetest"
)

func newFakeS3(t *testing.T) (store.Storage, *bytes.Buffer) {
	backend := s3mem.New()
	require.NoError(t, backend.CreateBucket("my-hop"))
	srv := httptest.NewServer(gofakes3.New(backend).Server())
	t.Cleanup(srv.Close)
	trace := &bytes.Buffer{}
	s, err := store.NewS3(context.Background(), store.Settings{
		Bucket: "my-hop", Region: "us-east-1", Endpoint: srv.URL,
		AccessKeyID: "AKIATEST", SecretAccessKey: "very-secret-value", Trace: trace,
	})
	require.NoError(t, err)
	return s, trace
}

func TestS3ContractAgainstGofakes3(t *testing.T) {
	storetest.Contract(t, func(t *testing.T) store.Storage {
		if strings.HasSuffix(t.Name(), "/IfNoneMatch") {
			t.Skip("gofakes3 does not implement conditional writes; covered by MinIO integration test")
		}
		s, _ := newFakeS3(t)
		return s
	})
}

func TestS3MultipartUpload(t *testing.T) {
	s, trace := newFakeS3(t)
	data := bytes.Repeat([]byte("0123456789abcdef"), (20<<20)/16)
	require.NoError(t, s.Put(context.Background(), "big.bin", bytes.NewReader(data), int64(len(data)), store.PutOptions{}))
	info, err := s.Head(context.Background(), "big.bin")
	require.NoError(t, err)
	assert.EqualValues(t, len(data), info.Size)
	assert.NotContains(t, trace.String(), "very-secret-value")
	assert.Contains(t, trace.String(), "s3: PutObject my-hop/big.bin")
}

func TestS3DeleteBatches(t *testing.T) {
	s, _ := newFakeS3(t)
	keys := make([]string, store.Limits.DeleteBatch+5)
	for i := range keys {
		keys[i] = "k/" + strconv.Itoa(i)
		require.NoError(t, s.Put(context.Background(), keys[i], strings.NewReader("1"), 1, store.PutOptions{}))
	}
	require.NoError(t, s.Delete(context.Background(), keys...))
	objs, _, err := s.List(context.Background(), "k/", "", store.Limits.NoLimit)
	require.NoError(t, err)
	assert.Empty(t, objs)
}
