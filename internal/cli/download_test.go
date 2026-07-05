package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/underhax/audiobook-tools/internal/core"
)

type mockBookDownloader struct {
	err error
}

func (m *mockBookDownloader) DownloadBook(_ context.Context, _, _ string, _, _ bool, _ int) (*core.BookInfo, []core.Chapter, string, error) {
	if m.err != nil {
		return nil, nil, "", m.err
	}
	return &core.BookInfo{}, []core.Chapter{}, "test_dir_m", nil
}

func TestDefaultDownloader(t *testing.T) {
	d := defaultDownloader(1)
	if d == nil {
		t.Error("expected non-nil downloader")
	}
}

func TestRunDownload(t *testing.T) {
	tests := []struct {
		setupMock func()
		name      string
		urlVal    string
		errStr    string
		extraArgs []string
		debugFlag bool
		wantErr   bool
	}{
		{
			name:      "download help",
			extraArgs: []string{"-h"},
			wantErr:   false,
		},
		{
			name:      "download bad flag",
			extraArgs: []string{"-invalid"},
			wantErr:   true,
			errStr:    "parse ",
		},
		{
			name:    "no url flag",
			wantErr: true,
			errStr:  "-url flag is required",
		},
		{
			name:    "download failed",
			urlVal:  "http://deti-online.com/book",
			wantErr: true,
			errStr:  "download failed: mock dl error",
			setupMock: func() {
				newDownloader = func(_ int) bookDownloader {
					return &mockBookDownloader{err: errors.New("mock dl error")}
				}
			},
		},
		{
			name:      "download success debug flag",
			urlVal:    "http://deti-online.com/book",
			debugFlag: true,
			wantErr:   false,
			setupMock: func() {
				newDownloader = func(_ int) bookDownloader {
					return &mockBookDownloader{err: nil}
				}
				osSetenv = func(_, _ string) error { return nil }
			},
		},
		{
			name:      "download success debug flag error",
			urlVal:    "http://deti-online.com/book_debug_err",
			debugFlag: true,
			wantErr:   true,
			errStr:    "set debug env: mock setenv error",
			setupMock: func() {
				newDownloader = func(_ int) bookDownloader {
					return &mockBookDownloader{err: nil}
				}
				osSetenv = func(_, _ string) error { return errors.New("mock setenv error") }
			},
		},
		{
			name:    "download success no m4b",
			urlVal:  "http://deti-online.com/book_2",
			wantErr: false,
			setupMock: func() {
				newDownloader = func(_ int) bookDownloader {
					return &mockBookDownloader{err: nil}
				}
			},
		},
		{
			name:      "download success with m4b",
			urlVal:    "http://deti-online.com/book_3",
			extraArgs: []string{"-m4b"},
			wantErr:   false,
			setupMock: func() {
				newDownloader = func(_ int) bookDownloader {
					return &mockBookDownloader{err: nil}
				}
				m4bCheckDependencies = func() error { return nil }
				m4bBuild = func(_ context.Context, _ *core.BookInfo, _ []core.Chapter, _ string, _ bool) (string, error) {
					return "out1.m4b", nil
				}
				m4bCleanIntermediateFiles = func(_ string) error { return nil }
			},
		},
		{
			name:      "download success with m4b error",
			urlVal:    "http://deti-online.com/m4berror",
			extraArgs: []string{"-m4b"},
			wantErr:   true,
			errStr:    "builder execution failed: missing dependencies: mock deps error",
			setupMock: func() {
				newDownloader = func(_ int) bookDownloader {
					return &mockBookDownloader{err: nil}
				}
				m4bCheckDependencies = func() error { return errors.New("mock deps error") }
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origDownloader := newDownloader
			origCheckDeps := m4bCheckDependencies
			origBuild := m4bBuild
			origClean := m4bCleanIntermediateFiles
			origSetenv := osSetenv

			defer func() {
				newDownloader = origDownloader
				m4bCheckDependencies = origCheckDeps
				m4bBuild = origBuild
				m4bCleanIntermediateFiles = origClean
				osSetenv = origSetenv
			}()

			if tt.setupMock != nil {
				tt.setupMock()
			}

			var runArgs []string
			if tt.urlVal != "" {
				runArgs = append(runArgs, "-url", tt.urlVal)
			}
			if tt.debugFlag {
				runArgs = append(runArgs, "-debug")
			}
			runArgs = append(runArgs, tt.extraArgs...)

			var out bytes.Buffer
			err := RunDownload(runArgs, &out)

			if (err != nil) != tt.wantErr {
				t.Fatalf("RunDownload() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("RunDownload() error = %v, want err string %q", err, tt.errStr)
			}
		})
	}
}

type downloadBadWriter struct {
	failOnCount int
	count       int
}

func (w *downloadBadWriter) Write(p []byte) (int, error) {
	if w.count >= w.failOnCount {
		return 0, errors.New("mock write error")
	}
	w.count++
	return len(p), nil
}

func TestRunDownloadBadWriter(t *testing.T) {
	origDownloader := newDownloader
	defer func() { newDownloader = origDownloader }()

	newDownloader = func(_ int) bookDownloader {
		return &mockBookDownloader{err: nil}
	}

	args := []string{"-url", "http://deti-online.com/badwriter"}
	writer := &downloadBadWriter{failOnCount: 0}
	err := RunDownload(args, writer)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "write output: mock write error") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestHandleDownloadError(t *testing.T) {
	tests := []struct {
		err       error
		name      string
		targetDir string
		errStr    string
		wantErr   bool
	}{
		{
			name:      "auth error",
			err:       &core.AuthError{ProviderName: "Test", ProviderID: "test_id", EnvVar: "TEST_TOKEN"},
			targetDir: "",
			wantErr:   false,
		},
		{
			name:      "api error",
			err:       errors.New("some prefix API error: check your token or subscription"),
			targetDir: "test_dir_1",
			wantErr:   false,
		},
		{
			name:      "generic error",
			err:       errors.New("mock download error"),
			targetDir: "",
			wantErr:   true,
			errStr:    "download failed: mock download error",
		},
		{
			name:      "paid books error",
			err:       errors.New("some prefix paid books from knigavuhe.org are not supported"),
			targetDir: "",
			wantErr:   false,
		},
		{
			name:      "subscription error",
			err:       errors.New("some prefix current subscription does not allow listening to this book (check your token or subscription)"),
			targetDir: "",
			wantErr:   false,
		},
		{
			name:      "prepare directory error",
			err:       errors.New("prepare directory: permission denied"),
			targetDir: "some_dir",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := handleDownloadError(tt.err, tt.targetDir, &out)
			if (err != nil) != tt.wantErr {
				t.Fatalf("handleDownloadError() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("handleDownloadError() error = %v, want err string %q", err, tt.errStr)
			}
		})
	}
}

func TestHandleDownloadErrorBadWriter(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		targetDir   string
		failOnCount int
		mockStderr  bool
	}{
		{
			name:        "auth error stderr fail",
			err:         &core.AuthError{ProviderName: "Test", ProviderID: "test_id", EnvVar: "TEST_TOKEN"},
			targetDir:   "",
			failOnCount: 0,
			mockStderr:  true,
		},
		{
			name:        "api error stderr fail",
			err:         errors.New("API error: testing"),
			targetDir:   "",
			failOnCount: 0,
			mockStderr:  true,
		},
		{
			name:        "api error out fail",
			err:         errors.New("API error: testing"),
			targetDir:   "test_dir_2",
			failOnCount: 0,
			mockStderr:  false,
		},
		{
			name:        "paid books stderr fail",
			err:         errors.New("paid books from knigavuhe.org are not supported"),
			targetDir:   "",
			failOnCount: 0,
			mockStderr:  true,
		},
		{
			name:        "prepare directory stderr fail",
			err:         errors.New("prepare directory: permission denied"),
			targetDir:   "some_dir",
			failOnCount: 0,
			mockStderr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origStderr := stderrWriter
			defer func() { stderrWriter = origStderr }()

			writer := &downloadBadWriter{failOnCount: tt.failOnCount}
			var out io.Writer
			if tt.mockStderr {
				stderrWriter = writer
				out = &bytes.Buffer{}
			} else {
				stderrWriter = &bytes.Buffer{}
				out = writer
			}

			err := handleDownloadError(tt.err, tt.targetDir, out)
			if err == nil {
				t.Errorf("expected write output error, got nil")
			} else if !strings.Contains(err.Error(), "write output") {
				t.Errorf("expected write output error, got %v", err)
			}
		})
	}
}

func TestExecuteBuilder(t *testing.T) {
	tests := []struct {
		setupMock func()
		name      string
		errStr    string
		clean     bool
		wantErr   bool
	}{
		{
			name:  "success",
			clean: true,
		},
		{
			name:    "absolute path error",
			clean:   true,
			wantErr: true,
			errStr:  "get absolute path: mock abs error",
			setupMock: func() {
				filepathAbs = func(_ string) (string, error) {
					return "", errors.New("mock abs error")
				}
			},
		},
		{
			name:    "deps error",
			clean:   false,
			wantErr: true,
			errStr:  "missing dependencies: mock deps error",
			setupMock: func() {
				m4bCheckDependencies = func() error { return errors.New("mock deps error") }
			},
		},
		{
			name:    "build error",
			clean:   false,
			wantErr: true,
			errStr:  "build m4b failed: mock build error",
			setupMock: func() {
				m4bBuild = func(_ context.Context, _ *core.BookInfo, _ []core.Chapter, _ string, _ bool) (string, error) {
					return "", errors.New("mock build error")
				}
			},
		},
		{
			name:    "clean error",
			clean:   true,
			wantErr: true,
			errStr:  "cleanup failed: mock clean error",
			setupMock: func() {
				m4bCleanIntermediateFiles = func(_ string) error { return errors.New("mock clean error") }
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origCheckDeps := m4bCheckDependencies
			origBuild := m4bBuild
			origClean := m4bCleanIntermediateFiles
			origAbs := filepathAbs

			defer func() {
				m4bCheckDependencies = origCheckDeps
				m4bBuild = origBuild
				m4bCleanIntermediateFiles = origClean
				filepathAbs = origAbs
			}()

			m4bCheckDependencies = func() error { return nil }
			m4bBuild = func(_ context.Context, _ *core.BookInfo, _ []core.Chapter, _ string, _ bool) (string, error) {
				return "out2.m4b", nil
			}
			m4bCleanIntermediateFiles = func(_ string) error { return nil }
			filepathAbs = func(path string) (string, error) { return "/mock/abs/" + path, nil }

			if tt.setupMock != nil {
				tt.setupMock()
			}

			var out bytes.Buffer
			info := &core.BookInfo{}
			chapters := []core.Chapter{}
			err := executeBuilder(context.Background(), info, chapters, "test_dir_3", tt.clean, false, &out)

			if (err != nil) != tt.wantErr {
				t.Fatalf("executeBuilder() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("executeBuilder() error = %v, want err string %q", err, tt.errStr)
			}
		})
	}
}

func TestExecuteBuilderBadWriter(t *testing.T) {
	tests := []struct {
		name        string
		failOnCount int
	}{
		{"fail on 0", 0},
		{"fail on 1", 1},
		{"fail on 2", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origCheckDeps := m4bCheckDependencies
			origBuild := m4bBuild
			origClean := m4bCleanIntermediateFiles
			origAbs := filepathAbs

			defer func() {
				m4bCheckDependencies = origCheckDeps
				m4bBuild = origBuild
				m4bCleanIntermediateFiles = origClean
				filepathAbs = origAbs
			}()

			m4bCheckDependencies = func() error { return nil }
			m4bBuild = func(_ context.Context, _ *core.BookInfo, _ []core.Chapter, _ string, _ bool) (string, error) {
				return "out3.m4b", nil
			}
			m4bCleanIntermediateFiles = func(_ string) error { return nil }
			filepathAbs = func(path string) (string, error) { return "/mock/abs/" + path, nil }

			writer := &downloadBadWriter{failOnCount: tt.failOnCount}
			info := &core.BookInfo{}
			chapters := []core.Chapter{}
			err := executeBuilder(context.Background(), info, chapters, "test_dir_4", true, false, writer)

			if err == nil {
				if writer.count <= tt.failOnCount {
					return
				}
				t.Errorf("expected write output error, got nil")
			} else if !strings.Contains(err.Error(), "write output") {
				t.Errorf("expected write output error, got %v", err)
			}
		})
	}
}
