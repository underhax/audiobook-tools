package m4b

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanIntermediateFiles(t *testing.T) {
	tempDir := t.TempDir()

	mp3Path := filepath.Join(tempDir, "001 Ch 1.mp3")
	m4aPath := filepath.Join(tempDir, "002.m4a")
	metaPath := filepath.Join(tempDir, "chapters.ffmeta")
	concatPath := filepath.Join(tempDir, "ffconcat.txt")

	if err := os.WriteFile(mp3Path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m4aPath, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaPath, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(concatPath, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := CleanIntermediateFiles(tempDir); err != nil {
		t.Fatalf("clean failed: %v", err)
	}

	if _, err := os.Stat(mp3Path); !os.IsNotExist(err) {
		t.Error("mp3 file not deleted")
	}
	if _, err := os.Stat(m4aPath); !os.IsNotExist(err) {
		t.Error("m4a file not deleted")
	}
	if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
		t.Error("meta file not deleted")
	}
}

func TestCleanIntermediateFiles_ReadDirError(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := CleanIntermediateFiles(filePath)
	if err == nil {
		t.Error("expected error reading a file as directory, got nil")
	}
}

func mockRemoveError(_ string) error {
	return errors.New("mock remove error")
}

func TestCleanIntermediateFiles_RemoveErrors(t *testing.T) {
	tempDir := t.TempDir()

	mp3Path := filepath.Join(tempDir, "test.mp3")
	metaPath := filepath.Join(tempDir, "cover.jpg")

	if err := os.WriteFile(mp3Path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaPath, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	origRemoveFile := removeFile
	removeFile = mockRemoveError
	defer func() { removeFile = origRemoveFile }()

	err := CleanIntermediateFiles(tempDir)
	if err != nil {
		t.Errorf("expected nil from CleanIntermediateFiles since remove errors are logged, got: %v", err)
	}
}
