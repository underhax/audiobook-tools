package m4b

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractChaptersFromDir(t *testing.T) {
	outDir := t.TempDir()

	_, errEmpty := ExtractChaptersFromDir(outDir)
	if errEmpty == nil {
		t.Error("expected error for empty directory, got nil")
	}

	files := []string{
		"001 Intro.mp3",
		"002 Chapter 1.m4a",
		"003_NoSpace.mp3",
		"ignore.txt",
		"004 .mp3",
	}

	for _, f := range files {
		path := filepath.Join(outDir, f)
		if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
	}

	subDir := filepath.Join(outDir, "subfolder.mp3")
	if err := os.MkdirAll(subDir, 0o750); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	chapters, err := ExtractChaptersFromDir(outDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chapters) != 4 {
		t.Fatalf("expected 4 chapters, got %d", len(chapters))
	}

	expected := []string{"Intro", "Chapter 1", "003_NoSpace", ""}
	for i, c := range chapters {
		if c.Title != expected[i] {
			t.Errorf("expected chapter %d to be %q, got %q", i, expected[i], c.Title)
		}
	}
}

func TestExtractChaptersFromDir_ReadError(t *testing.T) {
	outDir := t.TempDir()
	filePath := filepath.Join(outDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("write file error: %v", err)
	}

	_, err := ExtractChaptersFromDir(filePath)
	if err == nil {
		t.Error("expected error when reading a file as directory, got nil")
	}
}
