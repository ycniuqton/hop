package store

import (
	"bytes"
	"context"
	"io"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"
)

// Fake is an in-memory Storage with S3 listing semantics, used by unit tests.
type Fake struct {
	mu      sync.Mutex
	objects map[string]fakeObject
	Now     func() time.Time
	Broad   bool
	Missing bool
}

type fakeObject struct {
	data []byte
	info Object
}

// NewFake returns an empty in-memory store.
func NewFake() *Fake {
	return &Fake{objects: map[string]fakeObject{}, Now: time.Now}
}

// Put stores the body; IfNoneMatch fails when the key exists.
func (f *Fake) Put(ctx context.Context, key string, r io.Reader, _ int64, opt PutOptions) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.objects[key]; exists && opt.IfNoneMatch {
		return ErrPreconditionFailed
	}
	f.objects[key] = fakeObject{data: data, info: Object{
		Key: key, Size: int64(len(data)), Modified: f.Now(), Meta: maps.Clone(opt.Meta),
	}}
	return ctx.Err()
}

// Get returns a reader over a copy of the stored body.
func (f *Fake) Get(_ context.Context, key string) (io.ReadCloser, Object, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objects[key]
	if !ok {
		return nil, Object{}, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(bytes.Clone(o.data))), f.snapshot(o), nil
}

// Head returns object info without the body.
func (f *Fake) Head(_ context.Context, key string) (Object, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objects[key]
	if !ok {
		return Object{}, ErrNotFound
	}
	return f.snapshot(o), nil
}

// Delete removes keys; missing keys are not an error, as in S3.
func (f *Fake) Delete(_ context.Context, keys ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, k := range keys {
		delete(f.objects, k)
	}
	return nil
}

// Copy duplicates an object including its metadata.
func (f *Fake) Copy(_ context.Context, src, dst string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objects[src]
	if !ok {
		return ErrNotFound
	}
	o.info = f.snapshot(o)
	o.info.Key, o.info.Modified, o.data = dst, f.Now(), bytes.Clone(o.data)
	f.objects[dst] = o
	return nil
}

// List mimics ListObjectsV2: sorted keys, common prefixes when a delimiter is set, limit counts both.
func (f *Fake) List(_ context.Context, prefix, delimiter string, limit int) ([]Object, []string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var objs []Object
	var dirs []string
	seenDir := map[string]bool{}
	for _, key := range f.sortedKeys(prefix) {
		if limit != Limits.NoLimit && len(objs)+len(dirs) >= limit {
			break
		}
		if dir, ok := f.commonPrefix(key, prefix, delimiter); ok {
			if !seenDir[dir] {
				seenDir[dir] = true
				dirs = append(dirs, dir)
			}
			continue
		}
		objs = append(objs, f.snapshot(f.objects[key]))
	}
	return objs, dirs, nil
}

func (f *Fake) sortedKeys(prefix string) []string {
	var keys []string
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

func (f *Fake) commonPrefix(key, prefix, delimiter string) (string, bool) {
	if delimiter == "" {
		return "", false
	}
	rest := key[len(prefix):]
	i := strings.Index(rest, delimiter)
	if i < 0 {
		return "", false
	}
	return prefix + rest[:i+len(delimiter)], true
}

func (f *Fake) snapshot(o fakeObject) Object {
	info := o.info
	info.Meta = maps.Clone(o.info.Meta)
	return info
}

// Download copies the body into w.
func (f *Fake) Download(ctx context.Context, key string, w io.WriterAt) (int64, error) {
	rc, _, err := f.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	data, _ := io.ReadAll(rc)
	n, err := w.WriteAt(data, 0)
	return int64(n), err
}

// CheckBucket fails only when the fake is configured as a missing bucket.
func (f *Fake) CheckBucket(context.Context) error {
	if f.Missing {
		return ErrNotFound
	}
	return nil
}

// CanListAllBuckets reports the configured key breadth.
func (f *Fake) CanListAllBuckets(context.Context) bool { return f.Broad }
