package utils

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type failWriter struct {
	writes      int
	failOnWrite int
}

func (w *failWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failOnWrite {
		return 0, errors.New("mock write error")
	}
	return len(p), nil
}

func TestPrintUnfinishedWarning_Success(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		suffix   string
		files    []string
	}{
		{
			name:   "writes header files and suffix",
			suffix: "Please finish downloading before building M4B.",
			files:  []string{"/tmp/chapter-alpha.tmp", "/tmp/chapter-beta.tmp"},
			expected: "Warning: Unfinished downloads:\n" +
				"  • /tmp/chapter-alpha.tmp\n" +
				"  • /tmp/chapter-beta.tmp\n" +
				"Please finish downloading before building M4B.\n",
		},
		{
			name:     "writes header only when files and suffix are empty",
			expected: "Warning: Unfinished downloads:\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			err := PrintUnfinishedWarning(&buf, tt.files, tt.suffix)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if buf.String() != tt.expected {
				t.Fatalf("expected output %q, got %q", tt.expected, buf.String())
			}
		})
	}
}

func TestPrintUnfinishedWarning_Errors(t *testing.T) {
	tests := []struct {
		name        string
		failOnWrite int
	}{
		{
			name:        "fails on header write",
			failOnWrite: 1,
		},
		{
			name:        "fails on file write",
			failOnWrite: 2,
		},
		{
			name:        "fails on suffix write",
			failOnWrite: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var files []string
			var suffix string

			switch tt.failOnWrite {
			case 1:
				files = []string{"/tmp/chapter-gamma.tmp"}
				suffix = "header suffix"
			case 2:
				files = []string{"/tmp/chapter-delta.tmp"}
				suffix = "file suffix"
			case 3:
				files = []string{"/tmp/chapter-epsilon.tmp"}
				suffix = "Please finish downloading before building M4B."
			default:
				t.Fatalf("unexpected failOnWrite: %d", tt.failOnWrite)
			}

			err := PrintUnfinishedWarning(&failWriter{failOnWrite: tt.failOnWrite}, files, suffix)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "write output:") {
				t.Fatalf("expected wrapped write error, got %v", err)
			}
			if !strings.Contains(err.Error(), "mock write error") {
				t.Fatalf("expected mock write error, got %v", err)
			}
		})
	}
}
