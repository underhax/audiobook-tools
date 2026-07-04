package utils

import (
	"context"
	"net/http"
	"slices"
	"testing"
)

func TestSetHeaders(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", http.NoBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	SetHeaders(req)

	got := req.Header.Get("User-Agent")
	if got == "" {
		t.Error("SetHeaders() did not set User-Agent")
	}

	if !slices.Contains(userAgents, got) {
		t.Errorf("SetHeaders() set unknown User-Agent: %q", got)
	}
}
