package m4b

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/underhax/audiobook-tools/internal/core"
)

func TestBuild(t *testing.T) {
	tempDir := t.TempDir()

	fakeFFprobe := filepath.Join(tempDir, "ffprobe")
	if err := os.WriteFile(fakeFFprobe, []byte("#!/bin/sh\necho 1.5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fakeFFprobe, os.FileMode(0o700)); err != nil {
		t.Fatal(err)
	}

	fakeFFmpeg := filepath.Join(tempDir, "ffmpeg")
	if err := os.WriteFile(fakeFFmpeg, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fakeFFmpeg, os.FileMode(0o700)); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

	info := &core.BookInfo{
		Title:         "Test Book",
		Author:        "Test Author",
		Narrator:      "Test Narrator",
		Description:   "Desc",
		PublishedYear: "2023",
	}
	chapters := []core.Chapter{
		{Title: "Chapter 1", URL: "/1.mp3"},
		{Title: "Chapter 2", URL: "/2.mp3"},
	}
	if err := os.WriteFile(filepath.Join(tempDir, "001 Chapter 1.mp3"), []byte("mp3data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "002 Chapter 2.mp3"), []byte("mp3data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "cover.jpg"), []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Build(context.Background(), info, chapters, tempDir, false)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	metaContent, err := os.ReadFile(filepath.Clean(filepath.Join(tempDir, "chapters.ffmeta")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(metaContent), "TIMEBASE=1/1000") {
		t.Error("chapters.ffmeta missing TIMEBASE")
	}

	concatContent, err := os.ReadFile(filepath.Clean(filepath.Join(tempDir, "ffconcat.txt")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(concatContent), "file '001 Chapter 1.m4a'") {
		t.Error("ffconcat.txt missing file 1")
	}
}

func TestCleanIntermediateFiles(t *testing.T) {
	tempDir := t.TempDir()

	mp3Path := filepath.Join(tempDir, "001 Ch 1.mp3")
	metaPath := filepath.Join(tempDir, "chapters.ffmeta")
	concatPath := filepath.Join(tempDir, "ffconcat.txt")

	if err := os.WriteFile(mp3Path, []byte("data"), 0o600); err != nil {
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
	if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
		t.Error("meta file not deleted")
	}
}
