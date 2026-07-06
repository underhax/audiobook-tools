package httputil

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
		{199, ActionAbort},
		{100, ActionAbort},
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
	if err != nil {
		t.Fatalf("unexpected error for 404: %v", err)
	}
	defer func() {
		errClose := resp.Body.Close()
		_ = errClose
	}()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
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
			TestMode:   true,
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
			TestMode:   true,
		},
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

	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

type errReader struct{}

func (errReader) Read(_ []byte) (n int, err error) {
	return 0, errors.New("mock err")
}

func TestCalculateBackoff(t *testing.T) {
	if got := CalculateBackoff(ActionRetry, 1, time.Second); got != time.Second {
		t.Errorf("expected 1s, got %v", got)
	}
	if got := CalculateBackoff(ActionRateLimit, 5, 2*time.Second); got != 30*time.Second {
		t.Errorf("expected 30s, got %v", got)
	}
	if got := CalculateBackoff(ActionSuccess, 1, time.Second); got != 0 {
		t.Errorf("expected 0, got %v", got)
	}
}

func TestGetJitterDuration(t *testing.T) {
	origReader := rand.Reader
	defer func() { rand.Reader = origReader }()
	rand.Reader = errReader{}
	if got := getJitterDuration(); got != 300*time.Millisecond {
		t.Errorf("expected 300ms, got %v", got)
	}
}

func TestSleepContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepContext(ctx, time.Minute); err == nil {
		t.Error("expected err, got nil")
	}
}

func TestParseRetryAfter(t *testing.T) {
	if got := parseRetryAfter("", time.Second); got != time.Second {
		t.Errorf("expected 1s, got %v", got)
	}
	if got := parseRetryAfter("invalid", time.Second); got != time.Second {
		t.Errorf("expected 1s, got %v", got)
	}
	if got := parseRetryAfter("-1", time.Second); got != time.Second {
		t.Errorf("expected 1s, got %v", got)
	}
	if got := parseRetryAfter("5", time.Second); got != 5*time.Second {
		t.Errorf("expected 5s, got %v", got)
	}
}

func TestRetryTransport_TestMode(t *testing.T) {
	tr := &RetryTransport{TestMode: true}
	err := tr.sleep(context.Background(), time.Minute)
	if err != nil {
		t.Errorf("expected nil err, got %v", err)
	}
}

type failRoundTripper struct{}

func (f failRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("network fail")
}

func TestRetryTransport_MaxRetriesFail(t *testing.T) {
	tr := &RetryTransport{Base: failRoundTripper{}, MaxRetries: 1, TestMode: true}
	req, errReq := http.NewRequestWithContext(context.Background(), "GET", "http://example.net", http.NoBody)
	if errReq != nil {
		t.Fatal(errReq)
	}
	resp, err := tr.RoundTrip(req)
	if resp != nil {
		errClose := resp.Body.Close()
		_ = errClose
	}
	if err == nil || !strings.Contains(err.Error(), "max retries reached") {
		t.Fatalf("expected max retries err, got %v", err)
	}
}

func TestRetryTransport_ContextCancelDuringRetry(t *testing.T) {
	tr := &RetryTransport{Base: failRoundTripper{}, MaxRetries: 1, TestMode: true}
	ctx, cancel := context.WithCancel(context.Background())
	req, errReq := http.NewRequestWithContext(ctx, "GET", "http://example.org", http.NoBody)
	if errReq != nil {
		t.Fatal(errReq)
	}
	cancel()
	resp, err := tr.RoundTrip(req)
	if resp != nil {
		errClose := resp.Body.Close()
		_ = errClose
	}
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

type cancelRoundTripper struct {
	cancel context.CancelFunc
}

func (c cancelRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	c.cancel()
	return nil, errors.New("network fail")
}

func TestRetryTransport_ContextCancelAfterFail(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tr := &RetryTransport{Base: cancelRoundTripper{cancel: cancel}, MaxRetries: 2, TestMode: true}
	req, errReq := http.NewRequestWithContext(ctx, "GET", "http://test.example.com", http.NoBody)
	if errReq != nil {
		t.Fatal(errReq)
	}

	resp, err := tr.RoundTrip(req)
	if resp != nil {
		errClose := resp.Body.Close()
		_ = errClose
	}
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestRetryTransport_MaxRetries500(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	tr := &RetryTransport{MaxRetries: 1, TestMode: true}
	req, errReq := http.NewRequestWithContext(context.Background(), "GET", ts.URL, http.NoBody)
	if errReq != nil {
		t.Fatal(errReq)
	}
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp != nil {
		errClose := resp.Body.Close()
		_ = errClose
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500")
	}
}

type statusRoundTripper struct {
	cancel context.CancelFunc
}

func (s statusRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	s.cancel()
	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

func TestRetryTransport_ContextCancelAfter500(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tr := &RetryTransport{Base: statusRoundTripper{cancel: cancel}, MaxRetries: 2, TestMode: true}
	req, errReq := http.NewRequestWithContext(ctx, "GET", "http://test.example.org", http.NoBody)
	if errReq != nil {
		t.Fatal(errReq)
	}

	resp, err := tr.RoundTrip(req)
	if resp != nil {
		errClose := resp.Body.Close()
		_ = errClose
	}
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context canceled, got %v", err)
	}
}
