// Package m4b contains logic for assembling audiobooks using ffmpeg.
package m4b

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/underhax/audiobook-tools/internal/core"
	"github.com/underhax/audiobook-tools/pkg/utils"
	"golang.org/x/sync/errgroup"
)

// CheckDependencies verifies that ffmpeg and ffprobe are available in the system PATH.
func CheckDependencies() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return errors.New("ffmpeg not found in PATH: please install ffmpeg (e.g. brew install ffmpeg)")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return errors.New("ffprobe not found in PATH: please install ffmpeg")
	}
	return nil
}

// Build generates the final .m4b file from downloaded MP3s.
func Build(ctx context.Context, info *core.BookInfo, chapters []core.Chapter, targetDir string, debug bool) (string, error) {
	metaPath := filepath.Join(targetDir, "chapters.ffmeta")
	concatPath := filepath.Join(targetDir, "ffconcat.txt")

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return "", fmt.Errorf("read target dir: %w", err)
	}

	var mp3Files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".mp3") {
			mp3Files = append(mp3Files, e.Name())
		}
	}

	totalFiles := len(mp3Files)
	if totalFiles == 0 {
		return "", errors.New("no mp3 files found in target directory")
	}

	err = convertAllToM4A(ctx, targetDir, mp3Files)
	if err != nil {
		return "", fmt.Errorf("parallel conversion: %w", err)
	}

	fmt.Println("All files converted successfully. Assembling M4B...")

	err = generateConcatAndMetaFiles(ctx, targetDir, mp3Files, chapters)
	if err != nil {
		return "", err
	}

	return runFFmpeg(ctx, info, targetDir, concatPath, metaPath, debug)
}

func generateConcatAndMetaFiles(ctx context.Context, targetDir string, mp3Files []string, chapters []core.Chapter) error {
	metaPath := filepath.Join(targetDir, "chapters.ffmeta")
	concatPath := filepath.Join(targetDir, "ffconcat.txt")

	var metaBuilder strings.Builder
	var concatBuilder strings.Builder

	metaBuilder.WriteString(";FFMETADATA1\n")

	var offsetMs int64

	for i, file := range mp3Files {
		mp3Path := filepath.Join(targetDir, file)
		m4aName := strings.TrimSuffix(file, ".mp3") + ".m4a"
		m4aPath := filepath.Join(targetDir, m4aName)

		durationStr, err := getDurationSeconds(ctx, m4aPath)
		if err != nil {
			return fmt.Errorf("get duration for %s: %w", file, err)
		}

		durS, err := strconv.ParseFloat(strings.TrimSpace(durationStr), 64)
		if err != nil {
			return fmt.Errorf("parse duration %s: %w", durationStr, err)
		}

		durMs := int64(durS * 1000)
		endMs := offsetMs + durMs

		id3Title := ExtractID3Text(mp3Path, "TIT2")

		chapterTitle := ""
		if i < len(chapters) && chapters[i].Title != "" {
			chapterTitle = chapters[i].Title
		}

		if id3Title != "" {
			if scoreCyrillicString(id3Title) >= scoreCyrillicString(chapterTitle) || len(id3Title) > len(chapterTitle) {
				chapterTitle = id3Title
			}
		}

		if chapterTitle == "" {
			chapterTitle = strings.TrimSuffix(file, ".mp3")
		}

		titleEscaped := strings.ReplaceAll(chapterTitle, "=", "\\=")
		titleEscaped = strings.ReplaceAll(titleEscaped, ";", "\\;")
		titleEscaped = strings.ReplaceAll(titleEscaped, "#", "\\#")

		_, _ = fmt.Fprintf(&metaBuilder, "\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=%d\nEND=%d\ntitle=%s\n", offsetMs, endMs, titleEscaped)
		_, _ = fmt.Fprintf(&concatBuilder, "file '%s'\n", strings.ReplaceAll(m4aName, "'", "'\\''"))

		offsetMs = endMs
	}

	const perm = 0o644
	if err := os.WriteFile(metaPath, []byte(metaBuilder.String()), perm); err != nil {
		return fmt.Errorf("write ffmetadata: %w", err)
	}
	if err := os.WriteFile(concatPath, []byte(concatBuilder.String()), perm); err != nil {
		return fmt.Errorf("write ffconcat: %w", err)
	}

	return nil
}

func runFFmpeg(ctx context.Context, info *core.BookInfo, targetDir, concatPath, metaPath string, debug bool) (string, error) {
	outFileName := utils.SanitizeFilename(info.Title) + ".m4b"

	args := []string{
		"-y", "-hide_banner", "-loglevel", "warning", "-stats",
		"-f", "concat", "-safe", "0", "-i", concatPath,
		"-i", metaPath,
	}

	coverPath := filepath.Join(targetDir, "cover.jpg")
	hasCover := false
	if _, err := os.Stat(coverPath); err == nil {
		hasCover = true
		args = append(args, "-i", "cover.jpg")
	}

	args = append(args,
		"-map_metadata", "1",
		"-map", "0:a",
	)

	if hasCover {
		args = append(args, "-map", "2:v", "-c:v", "mjpeg", "-disposition:v", "attached_pic")
	}

	args = append(args,
		"-c:a", "copy",
		"-metadata", "title="+info.Title,
		"-metadata", "artist="+info.Author,
		"-metadata", "album_artist="+info.Author,
		"-metadata", "album="+info.Title,
		"-metadata", "composer="+info.Narrator,
	)

	if info.Description != "" {
		args = append(args, "-metadata", "comment="+info.Description)
	}
	if info.PublishedYear != "" {
		args = append(args, "-metadata", "date="+info.PublishedYear)
	}

	args = append(args, outFileName)

	cmd := exec.CommandContext(ctx, "ffmpeg")
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = targetDir

	if debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("ffmpeg execution failed: %w", err)
		}
	} else {
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("ffmpeg execution failed: %w\nOutput: %s", err, string(out))
		}
	}

	return filepath.Join(targetDir, outFileName), nil
}

func getDurationSeconds(ctx context.Context, filePath string) (string, error) {
	cmd := exec.CommandContext(ctx, "ffprobe")
	cmd.Args = append(cmd.Args, "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", filePath)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ffprobe failed for %s: %w", filePath, err)
	}
	return string(out), nil
}

func convertAllToM4A(ctx context.Context, targetDir string, mp3Files []string) error {
	totalFiles := len(mp3Files)
	var completed atomic.Int32
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())

	done := make(chan struct{})
	start := time.Now()
	go func() {
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				c := completed.Load()
				elapsed := time.Since(start).Round(time.Second)
				fmt.Printf("\r\033[K%s Converting... [%d/%d] (Elapsed: %s)", frames[i%len(frames)], c, totalFiles, elapsed)
				i++
			}
		}
	}()

	for _, file := range mp3Files {
		g.Go(func() error {
			mp3Path := filepath.Join(targetDir, file)
			m4aName := strings.TrimSuffix(file, ".mp3") + ".m4a"
			m4aPath := filepath.Join(targetDir, m4aName)

			needsConversion := true
			if stat, err := os.Stat(m4aPath); err == nil && stat.Size() > 0 {
				needsConversion = false
			}

			if needsConversion {
				args := []string{
					"-y", "-hide_banner", "-loglevel", "error",
					"-i", mp3Path,
					"-vn",
					"-c:a", "aac", "-b:a", "64k",
					m4aPath,
				}
				cmd := exec.CommandContext(gCtx, "ffmpeg")
				cmd.Args = append(cmd.Args, args...)
				out, err := cmd.CombinedOutput()
				if err != nil {
					return fmt.Errorf("failed to convert %s to m4a: %w\nOutput: %s", file, err, string(out))
				}
			}

			completed.Add(1)
			return nil
		})
	}

	err := g.Wait()
	close(done)
	fmt.Print("\r\033[K")
	if err != nil {
		return fmt.Errorf("errgroup wait: %w", err)
	}
	return nil
}
