package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRunUpdate(t *testing.T) {
	origUpdate := updaterUpdate
	defer func() { updaterUpdate = origUpdate }()

	tests := []struct {
		mockUpdate  func(context.Context, *http.Client, string) error
		name        string
		errContains string
		args        []string
		wantErr     bool
	}{
		{
			name:    "help_flag",
			args:    []string{"-h"},
			wantErr: false,
		},
		{
			name:        "bad_flag",
			args:        []string{"-unknown"},
			wantErr:     true,
			errContains: "parse flags",
		},
		{
			name:        "unexpected_positional_argument",
			args:        []string{"unexpected"},
			wantErr:     true,
			errContains: "unexpected argument: unexpected",
		},
		{
			name: "update_success",
			args: []string{},
			mockUpdate: func(_ context.Context, _ *http.Client, _ string) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "update_error",
			args: []string{},
			mockUpdate: func(_ context.Context, _ *http.Client, _ string) error {
				return errors.New("mock network error")
			},
			wantErr:     true,
			errContains: "update failed: mock network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockUpdate != nil {
				updaterUpdate = tt.mockUpdate
			} else {
				updaterUpdate = origUpdate
			}

			var buf bytes.Buffer
			err := RunUpdate(tt.args, &buf)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RunUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
			}
		})
	}
}

type mockUpdateTransport struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockUpdateTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func TestDefaultUpdaterUpdate(t *testing.T) {
	t.Run("dev_version_error", func(t *testing.T) {
		err := defaultUpdaterUpdate(context.Background(), http.DefaultClient, "dev")
		if err == nil || !strings.Contains(err.Error(), "update not available for development builds") {
			t.Fatalf("expected development build error, got %v", err)
		}
	})

	t.Run("success_same_version", func(t *testing.T) {
		client := &http.Client{
			Transport: &mockUpdateTransport{
				roundTripFunc: func(_ *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v1.0.0"}`)),
					}, nil
				},
			},
		}

		err := defaultUpdaterUpdate(context.Background(), client, "v1.0.0")
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})
}
