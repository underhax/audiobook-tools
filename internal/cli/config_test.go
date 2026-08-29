package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/underhax/audiobook-tools/internal/config"
)

func TestDefaultConfigFunctions(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("AppData", tempDir)
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	_, err := defaultConfigLoad()
	if err != nil {
		t.Fatalf("defaultConfigLoad error: %v", err)
	}

	if err := defaultConfigSetOutputDir("/test/path"); err != nil {
		t.Fatalf("defaultConfigSetOutputDir error: %v", err)
	}

	if err := defaultConfigClearOutputDir(); err != nil {
		t.Fatalf("defaultConfigClearOutputDir error: %v", err)
	}

	appDir, errDir := os.UserConfigDir()
	if errDir != nil {
		t.Fatalf("user config dir: %v", errDir)
	}
	configPath := filepath.Join(appDir, "audiobook-tools", "config.json")
	if writeErr := os.WriteFile(configPath, []byte("broken-json"), 0o600); writeErr != nil {
		t.Fatalf("write broken config: %v", writeErr)
	}

	if _, err := defaultConfigLoad(); err == nil {
		t.Fatal("expected defaultConfigLoad error")
	}

	if err := defaultConfigSetOutputDir("/new/path"); err == nil {
		t.Fatal("expected defaultConfigSetOutputDir error")
	}

	if err := defaultConfigClearOutputDir(); err == nil {
		t.Fatal("expected defaultConfigClearOutputDir error")
	}
}

func TestRunConfig(t *testing.T) {
	origLoad := configLoad
	origSet := configSetOutputDir
	origClear := configClearOutputDir
	defer func() {
		configLoad = origLoad
		configSetOutputDir = origSet
		configClearOutputDir = origClear
	}()

	tests := []struct {
		mockLoad    func() (*config.Config, error)
		mockSet     func(string) error
		mockClear   func() error
		writer      io.Writer
		name        string
		errContains string
		outContains string
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
			args:        []string{"-unknown-flag"},
			wantErr:     true,
			errContains: "flag provided but not defined",
		},
		{
			name:        "unexpected_positional_argument",
			args:        []string{"unexpected"},
			wantErr:     true,
			errContains: "unexpected argument: unexpected",
		},
		{
			name:        "both_flags_error",
			args:        []string{"--out=/custom/path", "--clear-out"},
			wantErr:     true,
			errContains: "cannot specify both --out and --clear-out",
		},
		{
			name: "set_out_success",
			mockSet: func(_ string) error {
				return nil
			},
			outContains: "Default output directory set to: /custom/path",
			wantErr:     false,
		},
		{
			name: "set_out_error",
			mockSet: func(_ string) error {
				return errors.New("mock set error")
			},
			wantErr:     true,
			errContains: "set output directory: mock set error",
		},
		{
			name: "set_out_write_error",
			mockSet: func(_ string) error {
				return nil
			},
			writer:      badWriter{},
			wantErr:     true,
			errContains: "write output",
		},
		{
			name: "clear_out_success",
			mockClear: func() error {
				return nil
			},
			outContains: "Default output directory cleared.",
			wantErr:     false,
		},
		{
			name: "clear_out_error",
			mockClear: func() error {
				return errors.New("mock clear error")
			},
			wantErr:     true,
			errContains: "clear output directory: mock clear error",
		},
		{
			name: "clear_out_write_error",
			mockClear: func() error {
				return nil
			},
			writer:  badWriter{},
			wantErr: true,
		},
		{
			name: "view_config_load_error",
			args: []string{},
			mockLoad: func() (*config.Config, error) {
				return nil, errors.New("mock load error")
			},
			wantErr:     true,
			errContains: "load config: mock load error",
		},
		{
			name: "view_config_unset",
			args: []string{},
			mockLoad: func() (*config.Config, error) {
				return &config.Config{OutputDir: ""}, nil
			},
			outContains: "Default output directory: (not set)",
			wantErr:     false,
		},
		{
			name: "view_config_set",
			args: []string{},
			mockLoad: func() (*config.Config, error) {
				return &config.Config{OutputDir: "/saved/audiobooks"}, nil
			},
			outContains: "Default output directory: /saved/audiobooks",
			wantErr:     false,
		},
		{
			name: "view_config_write_error",
			args: []string{},
			mockLoad: func() (*config.Config, error) {
				return &config.Config{OutputDir: "/saved/audiobooks"}, nil
			},
			writer:  badWriter{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockLoad != nil {
				configLoad = tt.mockLoad
			} else {
				configLoad = origLoad
			}
			if tt.mockSet != nil {
				configSetOutputDir = tt.mockSet
			} else {
				configSetOutputDir = origSet
			}
			if tt.mockClear != nil {
				configClearOutputDir = tt.mockClear
			} else {
				configClearOutputDir = origClear
			}

			args := tt.args
			if args == nil {
				if tt.mockSet != nil {
					args = []string{"-out", "/custom/path"}
				} else if tt.mockClear != nil {
					args = []string{"-clear-out"}
				}
			}

			var buf bytes.Buffer
			outWriter := io.Writer(&buf)
			if tt.writer != nil {
				outWriter = tt.writer
			}

			err := RunConfig(args, outWriter)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RunConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
			}
			if tt.outContains != "" {
				if !strings.Contains(buf.String(), tt.outContains) {
					t.Errorf("expected output containing %q, got %s", tt.outContains, buf.String())
				}
			}
		})
	}
}
