package workspace

import (
	"context"
	"strings"
)

// Bulk moves and deletes whole key prefixes with server-side operations.
type Bulk struct {
	ws *Workspace
}

// MovePrefix copies every object under src to dst, then deletes the originals.
func (b Bulk) MovePrefix(ctx context.Context, src, dst string) (int, error) {
	objs, err := b.ws.listAll(ctx, src)
	if err != nil {
		return 0, err
	}
	err = Pool{Workers: Concurrency.Transfers}.Run(ctx, len(objs), func(ctx context.Context, i int) error {
		return b.ws.Storage.Copy(ctx, objs[i].Key, dst+strings.TrimPrefix(objs[i].Key, src))
	})
	if err != nil {
		return 0, err
	}
	keys := make([]string, len(objs))
	for i, o := range objs {
		keys[i] = o.Key
	}
	return len(keys), b.ws.Storage.Delete(ctx, keys...)
}

// DeletePrefix removes every object under prefix.
func (b Bulk) DeletePrefix(ctx context.Context, prefix string) (int, error) {
	objs, err := b.ws.listAll(ctx, prefix)
	if err != nil {
		return 0, err
	}
	keys := make([]string, len(objs))
	for i, o := range objs {
		keys[i] = o.Key
	}
	return len(keys), b.ws.Storage.Delete(ctx, keys...)
}
