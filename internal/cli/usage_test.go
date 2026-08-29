package cli

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestSetupUsage(t *testing.T) {
	var buf bytes.Buffer
	fs := flag.NewFlagSet("testcmd", flag.ContinueOnError)
	fs.SetOutput(&buf)

	fs.Bool("x", false, "short bool flag")
	fs.String("url", "https://example.com", "url flag")
	fs.Bool("multiline", false, "line1\nline2")
	fs.Int("zero", 0, "zero int flag")

	setupUsage(fs, "testcmd")
	fs.Usage()

	output := buf.String()
	if !strings.Contains(output, "Usage of testcmd:\n") {
		t.Errorf("expected usage header, got: %s", output)
	}
	if !strings.Contains(output, "  -x\tshort bool flag") {
		t.Errorf("expected short flag format, got: %s", output)
	}
	if !strings.Contains(output, "  --url string") {
		t.Errorf("expected long flag format, got: %s", output)
	}
	if !strings.Contains(output, "(default https://example.com)") {
		t.Errorf("expected default value, got: %s", output)
	}
	if !strings.Contains(output, "line1\n    \tline2") {
		t.Errorf("expected multiline indent, got: %s", output)
	}
}

type failUsageWriter struct {
	failOn int
	count  int
}

func (w *failUsageWriter) Write(p []byte) (int, error) {
	w.count++
	if w.count >= w.failOn {
		return 0, errors.New("write error")
	}
	return len(p), nil
}

func TestSetupUsage_Errors(_ *testing.T) {
	w1 := &failUsageWriter{failOn: 1}
	fs1 := flag.NewFlagSet("test1", flag.ContinueOnError)
	fs1.SetOutput(w1)
	fs1.Bool("flag", false, "usage")
	setupUsage(fs1, "test1")
	fs1.Usage()

	w2 := &failUsageWriter{failOn: 2}
	fs2 := flag.NewFlagSet("test2", flag.ContinueOnError)
	fs2.SetOutput(w2)
	fs2.Bool("flag", false, "usage")
	setupUsage(fs2, "test2")
	fs2.Usage()
}
