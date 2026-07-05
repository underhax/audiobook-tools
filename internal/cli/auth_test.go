package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type badWriter struct{}

func (w badWriter) Write(_ []byte) (n int, err error) {
	return 0, errors.New("write error")
}

func TestRunAuth(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tests := []struct {
		name    string
		errStr  string
		args    []string
		wantErr bool
	}{
		{
			name:    "help flag",
			args:    []string{"-h"},
			wantErr: false,
		},
		{
			name:    "invalid flag",
			args:    []string{"-invalid"},
			wantErr: true,
			errStr:  "parse flags",
		},
		{
			name:    "no arguments",
			args:    []string{},
			wantErr: true,
			errStr:  "provider name is required",
		},
		{
			name:    "no token",
			args:    []string{"any_provider"},
			wantErr: true,
			errStr:  "token is required",
		},
		{
			name:    "invalid provider",
			args:    []string{"invalid_provider", "token123"},
			wantErr: true,
			errStr:  "unsupported provider: invalid_provider",
		},
		{
			name:    "success books_yandex",
			args:    []string{"books_yandex", "my_token"},
			wantErr: false,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			runAuthNormalTest(t, tt.args, tt.wantErr, tt.errStr)
		})

		if !tt.wantErr && len(tt.args) > 0 && tt.args[0] != "-h" {
			t.Run(tt.name+"_badwriter", func(t *testing.T) {
				runAuthBadWriterTest(t, tt.args)
			})
			t.Run(tt.name+"_save_error", func(t *testing.T) {
				runAuthSaveErrorTest(t, tt.args)
			})
		}
	}
}

func runAuthNormalTest(t *testing.T, args []string, wantErr bool, errStr string) {
	var out bytes.Buffer
	err := RunAuth(args, &out)

	if (err != nil) != wantErr {
		t.Fatalf("RunAuth() error = %v, wantErr %v", err, wantErr)
	}
	if wantErr && !strings.Contains(err.Error(), errStr) {
		t.Errorf("RunAuth() error = %v, want err string %q", err, errStr)
	}
}

func runAuthBadWriterTest(t *testing.T, args []string) {
	err := RunAuth(args, badWriter{})
	if err == nil || !strings.Contains(err.Error(), "write output") {
		t.Errorf("expected write output error, got %v", err)
	}
}

func runAuthSaveErrorTest(t *testing.T, args []string) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	var out bytes.Buffer
	err := RunAuth(args, &out)
	if err == nil || !strings.Contains(err.Error(), "save") {
		t.Errorf("expected save token error, got %v", err)
	}
}
