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
	if err := os.WriteFile(fakeFFmpeg, []byte("#!/bin/sh\nfor last; do true; done\ntouch \"$last\"\nexit 0\n"), 0o600); err != nil {
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

func TestCheckDependencies(t *testing.T) {
	tempDir := t.TempDir()

	t.Setenv("PATH", tempDir)

	err := CheckDependencies()
	if err == nil || !strings.Contains(err.Error(), "ffmpeg not found") {
		t.Errorf("expected ffmpeg not found error, got: %v", err)
	}

	fakeFFmpeg := filepath.Join(tempDir, "ffmpeg")
	err = os.WriteFile(fakeFFmpeg, []byte("#!/bin/sh\nexit 0\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFmpeg, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}

	err = CheckDependencies()
	if err == nil || !strings.Contains(err.Error(), "ffprobe not found") {
		t.Errorf("expected ffprobe not found error, got: %v", err)
	}

	fakeFFprobe := filepath.Join(tempDir, "ffprobe")
	err = os.WriteFile(fakeFFprobe, []byte("#!/bin/sh\nexit 0\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFprobe, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}

	err = CheckDependencies()
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestBuild_ReadDirError(t *testing.T) {
	_, err := Build(context.Background(), &core.BookInfo{}, nil, "/non/existent/dir/that/will/fail", false)
	if err == nil || !strings.Contains(err.Error(), "read target dir") {
		t.Errorf("expected read target dir error, got: %v", err)
	}
}

func TestBuild_EmptyDir(t *testing.T) {
	tempDir := t.TempDir()
	_, err := Build(context.Background(), &core.BookInfo{}, nil, tempDir, false)
	if err == nil || !strings.Contains(err.Error(), "no source audio files") {
		t.Errorf("expected no source files error, got: %v", err)
	}
}

func TestBuild_SubdirAndM4a(t *testing.T) {
	tempDir := t.TempDir()

	err := os.Mkdir(filepath.Join(tempDir, "subdir"), 0o750)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(tempDir, "standalone.m4a"), []byte("data"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	fakeFFprobe := filepath.Join(tempDir, "ffprobe")
	err = os.WriteFile(fakeFFprobe, []byte("#!/bin/sh\necho 1.5\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFprobe, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}

	fakeFFmpeg := filepath.Join(tempDir, "ffmpeg")
	err = os.WriteFile(fakeFFmpeg, []byte("#!/bin/sh\nfor last; do true; done\ntouch \"$last\"\nexit 0\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFmpeg, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

	info := &core.BookInfo{Title: "Book One"}
	_, err = Build(context.Background(), info, nil, tempDir, false)
	if err != nil {
		t.Errorf("expected success, got: %v", err)
	}
}

func TestBuild_ConvertError(t *testing.T) {
	tempDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tempDir, "test.mp3"), []byte("data"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	fakeFFprobe := filepath.Join(tempDir, "ffprobe")
	err = os.WriteFile(fakeFFprobe, []byte("#!/bin/sh\necho 1.5\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFprobe, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}

	fakeFFmpeg := filepath.Join(tempDir, "ffmpeg")
	err = os.WriteFile(fakeFFmpeg, []byte("#!/bin/sh\nexit 1\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFmpeg, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

	info := &core.BookInfo{Title: "Book Two"}
	_, err = Build(context.Background(), info, nil, tempDir, false)
	if err == nil || !strings.Contains(err.Error(), "parallel conversion") {
		t.Errorf("expected parallel conversion error, got: %v", err)
	}
}

func TestBuild_GenerateMetaError(t *testing.T) {
	tempDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tempDir, "test.mp3"), []byte("data"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	fakeFFprobe := filepath.Join(tempDir, "ffprobe")
	err = os.WriteFile(fakeFFprobe, []byte("#!/bin/sh\nexit 1\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFprobe, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}

	fakeFFmpeg := filepath.Join(tempDir, "ffmpeg")
	err = os.WriteFile(fakeFFmpeg, []byte("#!/bin/sh\nfor last; do true; done\ntouch \"$last\"\nexit 0\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFmpeg, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

	info := &core.BookInfo{Title: "Book Three"}
	_, err = Build(context.Background(), info, nil, tempDir, false)
	if err == nil || !strings.Contains(err.Error(), "get duration") {
		t.Errorf("expected get duration error, got: %v", err)
	}
}

func TestGenerateConcatAndMetaFiles_ParseDurationError(t *testing.T) {
	tempDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tempDir, "test_parse.mp3"), []byte("data"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	fakeFFprobe := filepath.Join(tempDir, "ffprobe")
	err = os.WriteFile(fakeFFprobe, []byte("#!/bin/sh\necho not_a_number\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFprobe, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

	err = generateConcatAndMetaFiles(context.Background(), tempDir, []string{"test_parse.mp3"}, nil)
	if err == nil || !strings.Contains(err.Error(), "parse duration") {
		t.Errorf("expected parse duration error, got: %v", err)
	}
}

func TestGenerateConcatAndMetaFiles_ID3TitleWins(t *testing.T) {
	tempDir := t.TempDir()

	id3Header := []byte("ID3\x03\x00\x00\x00\x00\x00\x10")
	frameHeader := []byte("TIT2\x00\x00\x00\x10\x00\x00")
	frameData := append([]byte{3}, []byte("Super Long Title\x00")...)
	validContent := make([]byte, 0, len(id3Header)+len(frameHeader)+len(frameData))
	validContent = append(validContent, id3Header...)
	validContent = append(validContent, frameHeader...)
	validContent = append(validContent, frameData...)

	testMp3 := filepath.Join(tempDir, "test_id3.mp3")
	err := os.WriteFile(testMp3, validContent, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	fakeFFprobe := filepath.Join(tempDir, "ffprobe")
	err = os.WriteFile(fakeFFprobe, []byte("#!/bin/sh\necho 1.5\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFprobe, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

	chapters := []core.Chapter{{Title: "Short"}}
	err = generateConcatAndMetaFiles(context.Background(), tempDir, []string{"test_id3.mp3"}, chapters)
	if err != nil {
		t.Errorf("expected success, got: %v", err)
	}

	metaContent, err := os.ReadFile(filepath.Clean(filepath.Join(tempDir, "chapters.ffmeta")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(metaContent), "title=Super Long Titl") {
		t.Errorf("expected ID3 title to win, got meta: %s", string(metaContent))
	}
}

func TestGenerateConcatAndMetaFiles_WriteErrors(t *testing.T) {
	tempDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tempDir, "test_err.mp3"), []byte("data"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	fakeFFprobe := filepath.Join(tempDir, "ffprobe")
	err = os.WriteFile(fakeFFprobe, []byte("#!/bin/sh\necho 1.5\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFprobe, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

	origWriteFile := writeFile
	defer func() { writeFile = origWriteFile }()

	writeFile = func(name string, data []byte, perm os.FileMode) error {
		if strings.HasSuffix(name, "chapters.ffmeta") {
			return os.ErrPermission
		}
		return defaultWriteFile(name, data, perm)
	}
	err = generateConcatAndMetaFiles(context.Background(), tempDir, []string{"test_err.mp3"}, nil)
	if err == nil || !strings.Contains(err.Error(), "write ffmetadata") {
		t.Errorf("expected write ffmetadata error, got: %v", err)
	}

	writeFile = func(name string, data []byte, perm os.FileMode) error {
		if strings.HasSuffix(name, "ffconcat.txt") {
			return os.ErrPermission
		}
		return defaultWriteFile(name, data, perm)
	}
	err = generateConcatAndMetaFiles(context.Background(), tempDir, []string{"test_err.mp3"}, nil)
	if err == nil || !strings.Contains(err.Error(), "write ffconcat") {
		t.Errorf("expected write ffconcat error, got: %v", err)
	}
}

func TestRunFFmpeg_SuccessAndMetadata(t *testing.T) {
	tempDir := t.TempDir()

	fakeFFmpeg := filepath.Join(tempDir, "ffmpeg")
	err := os.WriteFile(fakeFFmpeg, []byte("#!/bin/sh\nexit 0\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFmpeg, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

	info := &core.BookInfo{
		Title:       "Meta Book",
		Publisher:   "MyPub",
		Language:    "eng",
		Series:      "MySeries",
		Description: "Desc",
	}

	_, err = runFFmpeg(context.Background(), info, tempDir, "concat.txt", "meta.txt", false)
	if err != nil {
		t.Errorf("expected success, got: %v", err)
	}

	_, err = runFFmpeg(context.Background(), info, tempDir, "concat.txt", "meta.txt", true)
	if err != nil {
		t.Errorf("expected success, got: %v", err)
	}
}

func TestRunFFmpeg_Errors(t *testing.T) {
	tempDir := t.TempDir()

	fakeFFmpeg := filepath.Join(tempDir, "ffmpeg")
	err := os.WriteFile(fakeFFmpeg, []byte("#!/bin/sh\necho 'fake error output' >&2\nexit 1\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFmpeg, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

	info := &core.BookInfo{Title: "Error Book"}

	_, err = runFFmpeg(context.Background(), info, tempDir, "concat.txt", "meta.txt", false)
	if err == nil || !strings.Contains(err.Error(), "ffmpeg execution failed") {
		t.Errorf("expected ffmpeg execution failed error, got: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "fake error output") {
		t.Errorf("expected error output, got: %v", err)
	}

	_, err = runFFmpeg(context.Background(), info, tempDir, "concat.txt", "meta.txt", true)
	if err == nil || !strings.Contains(err.Error(), "ffmpeg execution failed") {
		t.Errorf("expected ffmpeg execution failed error, got: %v", err)
	}
}

func TestGetBitrate_Error(t *testing.T) {
	tempDir := t.TempDir()

	fakeFFprobe := filepath.Join(tempDir, "ffprobe")
	err := os.WriteFile(fakeFFprobe, []byte("#!/bin/sh\nexit 1\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFprobe, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

	_, err = getBitrate(context.Background(), "test.mp3")
	if err == nil || !strings.Contains(err.Error(), "ffprobe bitrate failed") {
		t.Errorf("expected ffprobe bitrate failed error, got: %v", err)
	}
}

func TestNeedsM4AConversion(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	m4aPath := filepath.Join(tempDir, "existing.m4a")
	err := os.WriteFile(m4aPath, []byte("data"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if needsM4AConversion(ctx, ".mp3", m4aPath, "src") {
		t.Error("expected false for mp3 with existing m4a")
	}

	fakeFFprobe := filepath.Join(tempDir, "ffprobe")
	err = os.WriteFile(fakeFFprobe, []byte("#!/bin/sh\nexit 1\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFprobe, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)
	if !needsM4AConversion(ctx, ".m4a", m4aPath, "src") {
		t.Error("expected true for m4a with getBitrate error")
	}

	err = os.WriteFile(fakeFFprobe, []byte("#!/bin/sh\necho not_a_number\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if !needsM4AConversion(ctx, ".m4a", m4aPath, "src") {
		t.Error("expected true for m4a with non-integer bitrate")
	}

	err = os.WriteFile(fakeFFprobe, []byte("#!/bin/sh\necho 64000\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if needsM4AConversion(ctx, ".m4a", m4aPath, "src") {
		t.Error("expected false for m4a with low bitrate")
	}

	err = os.WriteFile(fakeFFprobe, []byte("#!/bin/sh\necho 128000\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if !needsM4AConversion(ctx, ".m4a", m4aPath, "src") {
		t.Error("expected true for m4a with high bitrate")
	}

	if !needsM4AConversion(ctx, ".wav", m4aPath, "src") {
		t.Error("expected true for unknown extension")
	}
}

func TestProcessSingleAudioFile_EarlyExitAndRenameError(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	baseName := "test_file"
	srcFile := baseName + ".mp3"
	m4aName := baseName + ".m4a"
	m4aPath := filepath.Join(tempDir, m4aName)
	err := os.WriteFile(m4aPath, []byte("data"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	err = processSingleAudioFile(ctx, tempDir, srcFile)
	if err != nil {
		t.Errorf("expected no error for early exit, got: %v", err)
	}

	baseNameErr := "test_rename"
	srcFileErr := baseNameErr + ".mp3"
	m4aNameErr := baseNameErr + ".m4a"
	_ = filepath.Join(tempDir, m4aNameErr)

	fakeFFmpeg := filepath.Join(tempDir, "ffmpeg")
	err = os.WriteFile(fakeFFmpeg, []byte("#!/bin/sh\nexit 0\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fakeFFmpeg, os.FileMode(0o700))
	if err != nil {
		t.Fatal(err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

	err = processSingleAudioFile(ctx, tempDir, srcFileErr)
	if err == nil || !strings.Contains(err.Error(), "failed to rename temp file") {
		t.Errorf("expected rename error, got: %v", err)
	}
}

func TestDefaultWriteFile_Error(t *testing.T) {
	tempDir := t.TempDir()
	err := defaultWriteFile(tempDir, []byte("data"), 0o600)
	if err == nil || !strings.Contains(err.Error(), "write file error") {
		t.Errorf("expected write file error, got: %v", err)
	}
}
