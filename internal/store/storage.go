// Package store is the storage boundary: one small interface with S3 and in-memory implementations.
package store

import (
	"context"
	"errors"
	"io"
	"time"
)

// Object describes one stored object.
type Object struct {
	Key      string
	Size     int64
	Modified time.Time
	Meta     map[string]string
}

// PutOptions controls conditional writes and user metadata.
type PutOptions struct {
	IfNoneMatch bool
	Meta        map[string]string
}

// Storage is everything hop needs from an object store.
type Storage interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, opt PutOptions) error
	Get(ctx context.Context, key string) (io.ReadCloser, Object, error)
	Head(ctx context.Context, key string) (Object, error)
	Delete(ctx context.Context, keys ...string) error
	Copy(ctx context.Context, src, dst string) error
	List(ctx context.Context, prefix, delimiter string, limit int) (objs []Object, dirs []string, err error)
	Download(ctx context.Context, key string, w io.WriterAt) (int64, error)
	CheckBucket(ctx context.Context) error
	CanListAllBuckets(ctx context.Context) bool
}

var (
	ErrNotFound           = errors.New("not found")
	ErrPreconditionFailed = errors.New("precondition failed")
	ErrAccessDenied       = errors.New("access denied")
	ErrNetwork            = errors.New("network error")
	ErrNotSupported       = errors.New("not supported by the storage provider")
)

// Limits are the storage tuning values from spec section 8.
var Limits = struct {
	DeleteBatch       int
	ListPageMax       int
	MultipartPartSize int64
	UploadConcurrency int
	DownloadPartSize  int64
	DownloadWorkers   int
	CopyMaxSingle     int64
	CopyPartSize      int64
	NoLimit           int
}{
	DeleteBatch:       1000,
	ListPageMax:       1000,
	MultipartPartSize: 16 << 20,
	UploadConcurrency: 4,
	DownloadPartSize:  16 << 20,
	DownloadWorkers:   4,
	CopyMaxSingle:     5 << 30,
	CopyPartSize:      512 << 20,
	NoLimit:           0,
}
