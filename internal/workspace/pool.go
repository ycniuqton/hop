package workspace

import (
	"context"
	"sync"
)

// Pool runs indexed jobs with bounded concurrency and stops at the first error.
type Pool struct {
	Workers int
}

// Concurrency holds the worker counts from spec section 8.
var Concurrency = struct{ Transfers, Lookups int }{Transfers: 8, Lookups: 8}

// Run calls fn for every index in [0, n).
func (p Pool) Run(ctx context.Context, n int, fn func(ctx context.Context, i int) error) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < max(1, min(p.Workers, n)); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if err := fn(ctx, i); err != nil {
					cancel(err)
				}
			}
		}()
	}
	for i := 0; i < n && ctx.Err() == nil; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return context.Cause(ctx)
}
