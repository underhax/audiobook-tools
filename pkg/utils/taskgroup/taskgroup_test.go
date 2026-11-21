package taskgroup

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestGroup_Basic(t *testing.T) {
	tests := []struct {
		name    string
		limit   int
		tasks   int
		wantErr bool
	}{
		{
			name:    "unbounded success",
			limit:   0,
			tasks:   10,
			wantErr: false,
		},
		{
			name:    "bounded success",
			limit:   2,
			tasks:   6,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := New(tt.limit)
			var counter atomic.Int32

			for range tt.tasks {
				g.Go(func() error {
					counter.Add(1)
					return nil
				})
			}

			err := g.Wait()
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected err: %v", err)
			}
			if int(counter.Load()) != tt.tasks {
				t.Fatalf("expected %d completed tasks, got %d", tt.tasks, counter.Load())
			}
		})
	}
}

func TestGroup_ConcurrencyLimit(t *testing.T) {
	const limit = 3
	const total = 9

	g := New(limit)
	var active atomic.Int32
	var maxActive atomic.Int32

	for range total {
		g.Go(func() error {
			current := active.Add(1)
			for {
				prev := maxActive.Load()
				if current <= prev || maxActive.CompareAndSwap(prev, current) {
					break
				}
			}

			time.Sleep(10 * time.Millisecond)
			active.Add(-1)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		t.Fatalf("unexpected wait err: %v", err)
	}

	if peak := maxActive.Load(); peak > limit {
		t.Fatalf("max active goroutines exceeded limit: %d > %d", peak, limit)
	}
}

func TestGroup_SetLimit(t *testing.T) {
	g := &Group{}
	g.SetLimit(2)
	var count atomic.Int32

	for range 4 {
		g.Go(func() error {
			count.Add(1)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if count.Load() != 4 {
		t.Fatalf("expected 4 tasks, got %d", count.Load())
	}

	g.SetLimit(0)
	if g.sem != nil {
		t.Fatal("expected sem to be nil when limit <= 0")
	}
}

func TestGroup_Errors(t *testing.T) {
	errFirst := errors.New("first task error")
	errSecond := errors.New("second task error")

	g := New(1)
	g.Go(func() error {
		return errFirst
	})
	g.Go(func() error {
		return errSecond
	})

	err := g.Wait()
	if !errors.Is(err, errFirst) {
		t.Fatalf("expected errFirst, got %v", err)
	}
}

func TestWithContext(t *testing.T) {
	errFailed := errors.New("context task failed")

	t.Run("cancels on task error", func(t *testing.T) {
		g, ctx := WithContext(context.Background(), 2)
		g.Go(func() error {
			return errFailed
		})
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(500 * time.Millisecond):
				return errors.New("timed out waiting for context cancel")
			}
		})

		err := g.Wait()
		if !errors.Is(err, errFailed) {
			t.Fatalf("expected errFailed, got %v", err)
		}
	})

	t.Run("cancels on wait complete", func(t *testing.T) {
		g, ctx := WithContext(context.Background(), 1)
		g.Go(func() error {
			return nil
		})

		if err := g.Wait(); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if ctx.Err() == nil {
			t.Fatal("expected context to be cancelled after Wait()")
		}
	})
}
