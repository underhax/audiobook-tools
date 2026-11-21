// Package taskgroup provides synchronization for concurrent task execution with concurrency limits, error tracking, and context cancellation.
package taskgroup

import (
	"context"
	"sync"
)

// Group coordinates concurrent tasks with optional concurrency limits, first-error recording, and context cancellation.
type Group struct {
	err     error
	cancel  context.CancelFunc
	sem     chan struct{}
	wg      sync.WaitGroup
	errOnce sync.Once
}

// New initializes a Group with a concurrency limit. When limit <= 0, concurrency is unbounded.
func New(limit int) *Group {
	g := &Group{}
	g.SetLimit(limit)
	return g
}

// WithContext initializes a Group derived from the parent context with a concurrency limit.
// The derived context is canceled upon the first task error or when Wait completes.
func WithContext(ctx context.Context, limit int) (*Group, context.Context) {
	cCtx, cancel := context.WithCancel(ctx)
	g := &Group{
		cancel: cancel,
	}
	g.SetLimit(limit)
	return g, cCtx
}

// SetLimit configures the maximum number of concurrent goroutines. When limit <= 0, concurrency is unbounded.
func (g *Group) SetLimit(limit int) {
	if limit <= 0 {
		g.sem = nil
		return
	}
	g.sem = make(chan struct{}, limit)
}

// Go executes the provided function in a new goroutine respecting the concurrency limit.
func (g *Group) Go(f func() error) {
	if g.sem != nil {
		g.sem <- struct{}{}
	}
	g.wg.Add(1)

	go func() {
		defer func() {
			if g.sem != nil {
				<-g.sem
			}
			g.wg.Done()
		}()

		if err := f(); err != nil {
			g.errOnce.Do(func() {
				g.err = err
				if g.cancel != nil {
					g.cancel()
				}
			})
		}
	}()
}

// Wait blocks until all active tasks finish and returns the first encountered error.
func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.err
}
