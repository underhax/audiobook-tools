package httputil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDetermineAction(t *testing.T) {
	tests := []struct {
		code   int
		action ActionType
	}{
		{200, ActionSuccess},
		{404, ActionAbort},
		{429, ActionRateLimit},
		{408, ActionRetry},
		{500, ActionRetry},
		{503, ActionRetry},
		{522, ActionRetry},
	}
	for _, tt := range tests {
		if got := DetermineAction(tt.code); got != tt.action {
			t.Errorf("DetermineAction(%d) = %v, want %v", tt.code, got, tt.action)
		}
	}
}

func TestRetryTransport_Success(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &http.Client{
		Transport: &RetryTransport{},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		errClose := resp.Body.Close()
		_ = errClose
	}()

	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetryTransport_Abort(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := &http.Client{
		Transport: &RetryTransport{},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if resp != nil && resp.Body != nil {
		errClose := resp.Body.Close()
		_ = errClose
	}
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}

	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetryTransport_Retry(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &http.Client{
		Transport: &RetryTransport{
			MaxRetries: 3,
		},
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		errClose := resp.Body.Close()
		_ = errClose
	}()

	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetryTransport_RateLimit(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &http.Client{
		Transport: &RetryTransport{
			MaxRetries: 2,
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		errClose := resp.Body.Close()
		_ = errClose
	}()

	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}

	if time.Since(start) < 1*time.Second {
		t.Error("expected to sleep for at least 1 second due to Retry-After")
	}
}
