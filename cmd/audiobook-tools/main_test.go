package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestMainFunc(_ *testing.T) {
	osExit = func(_ int) {}
	defer func() { osExit = os.Exit }()

	os.Args = []string{os.Args[0], "-h"}
	main()
}

type runTestCase struct {
	name           string
	wantUnknownCmd string
	args           []string
	wantExitCode   int
	wantUsage      bool
	wantVersion    bool
	wantError      bool
}

func (tc *runTestCase) runTest(t *testing.T) {
	var stdout, stderr bytes.Buffer

	fullArgs := append([]string{"audiobook-tools"}, tc.args...)
	exitCode := run(fullArgs, &stdout, &stderr)

	if exitCode != tc.wantExitCode {
		t.Errorf("got exit code %d, want %d", exitCode, tc.wantExitCode)
	}

	outStr := stdout.String()
	errStr := stderr.String()

	if tc.wantUsage && !strings.Contains(outStr, "Usage:") && !strings.Contains(errStr, "Usage:") {
		t.Errorf("expected usage output, got stdout: %q, stderr: %q", outStr, errStr)
	}

	if tc.wantVersion && !strings.Contains(outStr, "audiobook-tools version") {
		t.Errorf("expected version output, got %q", outStr)
	}

	if tc.wantError && !strings.Contains(errStr, "Error:") {
		t.Errorf("expected error output, got %q", errStr)
	}

	if tc.wantUnknownCmd != "" {
		expected := "Unknown command: " + tc.wantUnknownCmd
		if !strings.Contains(errStr, expected) {
			t.Errorf("expected %q, got %q", expected, errStr)
		}
	}
}

type badWriter struct{}

func (w badWriter) Write(_ []byte) (n int, err error) {
	return 0, errors.New("write error")
}

func (tc *runTestCase) runBadWriterTest(_ *testing.T) {
	var bw badWriter
	fullArgs := append([]string{"audiobook-tools"}, tc.args...)
	run(fullArgs, bw, bw)
}

func TestRun(t *testing.T) {
	tests := []runTestCase{
		{
			name:         "no arguments",
			args:         []string{},
			wantExitCode: 1,
			wantUsage:    true,
		},
		{
			name:         "help command",
			args:         []string{"help"},
			wantExitCode: 0,
			wantUsage:    true,
		},
		{
			name:         "-h flag",
			args:         []string{"-h"},
			wantExitCode: 0,
			wantUsage:    true,
		},
		{
			name:         "--help flag",
			args:         []string{"--help"},
			wantExitCode: 0,
			wantUsage:    true,
		},
		{
			name:         "version command",
			args:         []string{"version"},
			wantExitCode: 0,
			wantVersion:  true,
		},
		{
			name:         "-v flag",
			args:         []string{"-v"},
			wantExitCode: 0,
			wantVersion:  true,
		},
		{
			name:         "--version flag",
			args:         []string{"--version"},
			wantExitCode: 0,
			wantVersion:  true,
		},
		{
			name:           "unknown command",
			args:           []string{"invalid-command"},
			wantExitCode:   1,
			wantUnknownCmd: "invalid-command",
			wantUsage:      true,
		},
		{
			name:         "download command triggers error without args",
			args:         []string{"download"},
			wantExitCode: 1,
			wantError:    true,
		},
		{
			name:         "auth command help",
			args:         []string{"auth", "-h"},
			wantExitCode: 0,
		},
		{
			name:         "build command triggers error without args",
			args:         []string{"build"},
			wantExitCode: 1,
			wantError:    true,
		},
	}

	for i := range tests {
		t.Run(tests[i].name, tests[i].runTest)
		t.Run(tests[i].name+"_badwriter", tests[i].runBadWriterTest)
	}
}
