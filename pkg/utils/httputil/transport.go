// Package httputil provides HTTP utilities such as retry transports.
package httputil

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"time"
)

// ActionType defines how the transport should handle the response.
type ActionType int

// ActionType constants decouple HTTP status codes for testable retry logic.
const (
	ActionRetry ActionType = iota
	ActionRateLimit
	ActionAbort
	ActionSuccess
)

// DetermineAction classifies the HTTP status code.
func DetermineAction(statusCode int) ActionType {
	switch {
	case statusCode >= 200 && statusCode < 400:
		return ActionSuccess
	case statusCode == 408:
		return ActionRetry
	case statusCode == 429:
		return ActionRateLimit
	case statusCode >= 520 && statusCode <= 530:
		return ActionRetry
	case statusCode >= 400 && statusCode < 500:
		return ActionAbort
	case statusCode >= 500:
		return ActionRetry
	default:
		return ActionAbort
	}
}

// CalculateBackoff computes the delay for the next attempt.
func CalculateBackoff(action ActionType, attempt int, baseDelay time.Duration) time.Duration {
	const maxDelay = 30 * time.Second
	switch action {
	case ActionRetry:
		return baseDelay
	case ActionRateLimit:
		shift := min(max(attempt, 0), 4)
		delay := baseDelay << uint(shift)
		if delay > maxDelay {
			return maxDelay
		}
		return delay
	default:
		return 0
	}
}

func getJitterDuration() time.Duration {
	n, err := rand.Int(rand.Reader, big.NewInt(500))
	if err != nil {
		return 300 * time.Millisecond
	}
	return time.Duration(n.Int64()+300) * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	select {
	case <-ctx.Done():
		timer.Stop()
		return fmt.Errorf("context canceled: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

func parseRetryAfter(headerValue string, defaultDelay time.Duration) time.Duration {
	if headerValue == "" {
		return defaultDelay
	}
	if parsedSecs, err := strconv.Atoi(headerValue); err == nil && parsedSecs > 0 {
		return time.Duration(parsedSecs) * time.Second
	}
	return defaultDelay
}

// RetryOption configures the RetryTransport.
type RetryOption func(*RetryTransport)

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(n int) RetryOption {
	return func(t *RetryTransport) {
		t.MaxRetries = n
	}
}

// WithBaseTransport sets the underlying round tripper.
func WithBaseTransport(base http.RoundTripper) RetryOption {
	return func(t *RetryTransport) {
		t.Base = base
	}
}

// NewRetryTransport creates a new RetryTransport with the provided options.
func NewRetryTransport(opts ...RetryOption) *RetryTransport {
	rt := &RetryTransport{
		MaxRetries: 3,
		Base:       http.DefaultTransport,
	}
	for _, opt := range opts {
		opt(rt)
	}
	return rt
}

// GetBaseTransport safely extracts the underlying transport from a round tripper.
// If the provided transport is a *RetryTransport, it returns its Base.
func GetBaseTransport(rt http.RoundTripper) http.RoundTripper {
	if retryRT, ok := rt.(*RetryTransport); ok {
		if retryRT.Base != nil {
			return retryRT.Base
		}
		return http.DefaultTransport
	}
	return rt
}

// RetryTransport wraps an http.RoundTripper with retry, backoff, and jitter logic.
type RetryTransport struct {
	Base       http.RoundTripper
	MaxRetries int
	TestMode   bool
}

func (t *RetryTransport) sleep(ctx context.Context, delay time.Duration) error {
	if t.TestMode {
		delay = time.Millisecond
	}
	return sleepContext(ctx, delay)
}

// RoundTrip executes a single HTTP transaction, returning a Response for the provided Request.
func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	maxRetries := t.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if errSleep := t.sleep(req.Context(), getJitterDuration()); errSleep != nil {
			return nil, errSleep
		}

		resp, err = base.RoundTrip(req)
		if err != nil {
			if attempt == maxRetries {
				return nil, fmt.Errorf("max retries reached (%d): %w", maxRetries, err)
			}
			if errSleep := t.sleep(req.Context(), CalculateBackoff(ActionRetry, attempt, 2*time.Second)); errSleep != nil {
				return nil, errSleep
			}
			continue
		}

		action := DetermineAction(resp.StatusCode)
		if action == ActionSuccess {
			return resp, nil
		}

		if action == ActionAbort {
			return resp, nil
		}

		if attempt == maxRetries {
			break
		}

		errClose := resp.Body.Close()
		_ = errClose

		delay := CalculateBackoff(action, attempt, 2*time.Second)
		if action == ActionRateLimit {
			delay = parseRetryAfter(resp.Header.Get("Retry-After"), delay)
		}

		if errSleep := t.sleep(req.Context(), delay); errSleep != nil {
			return nil, errSleep
		}
	}

	return resp, nil
}
