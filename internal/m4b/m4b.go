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

	var sourceFiles []string
	mp3Bases := make(map[string]bool)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".mp3") {
			base := e.Name()[:len(e.Name())-4]
			mp3Bases[base] = true
		}
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".mp3") {
			sourceFiles = append(sourceFiles, name)
		} else if strings.HasSuffix(lower, ".m4a") {
			base := name[:len(name)-4]
			if !mp3Bases[base] {
				sourceFiles = append(sourceFiles, name)
			}
		}
	}

	totalFiles := len(sourceFiles)
	if totalFiles == 0 {
		return "", errors.New("no source audio files (.mp3 or .m4a) found in target directory")
	}

	err = convertAllToM4A(ctx, targetDir, sourceFiles)
	if err != nil {
		return "", fmt.Errorf("parallel conversion: %w", err)
	}

	fmt.Println("All files converted successfully. Assembling M4B...")

	err = generateConcatAndMetaFiles(ctx, targetDir, sourceFiles, chapters)
	if err != nil {
		return "", err
	}

	return runFFmpeg(ctx, info, targetDir, concatPath, metaPath, debug)
}

func generateConcatAndMetaFiles(ctx context.Context, targetDir string, sourceFiles []string, chapters []core.Chapter) error {
	metaPath := filepath.Join(targetDir, "chapters.ffmeta")
	concatPath := filepath.Join(targetDir, "ffconcat.txt")

	var metaBuilder strings.Builder
	var concatBuilder strings.Builder

	metaBuilder.WriteString(";FFMETADATA1\n")

	var offsetMs int64

	for i, file := range sourceFiles {
		srcPath := filepath.Join(targetDir, file)
		ext := filepath.Ext(file)
		m4aName := strings.TrimSuffix(file, ext) + ".m4a"
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

		id3Title := ""
		if strings.EqualFold(ext, ".mp3") {
			id3Title = ExtractID3Text(srcPath, "TIT2")
		}

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
			chapterTitle = strings.TrimSuffix(file, ext)
		}

		titleEscaped := strings.ReplaceAll(chapterTitle, "=", "\\=")
		titleEscaped = strings.ReplaceAll(titleEscaped, ";", "\\;")
		titleEscaped = strings.ReplaceAll(titleEscaped, "#", "\\#")

		_, _ = fmt.Fprintf(&metaBuilder, "\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=%d\nEND=%d\ntitle=%s\n", offsetMs, endMs, titleEscaped)
		_, _ = fmt.Fprintf(&concatBuilder, "file '%s'\n", strings.ReplaceAll(m4aName, "'", "'\\''"))

		offsetMs = endMs
	}

	const perm = 0o644
	if err := writeFile(metaPath, []byte(metaBuilder.String()), perm); err != nil {
		return fmt.Errorf("write ffmetadata: %w", err)
	}
	if err := writeFile(concatPath, []byte(concatBuilder.String()), perm); err != nil {
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

	if info.Publisher != "" {
		args = append(args, "-metadata", "publisher="+info.Publisher)
	}
	if info.Language != "" {
		args = append(args, "-metadata", "language="+info.Language)
	} else {
		args = append(args, "-metadata", "language=rus")
	}
	if info.Series != "" {
		args = append(args, "-metadata", "grouping="+info.Series)
	}

	if info.Description != "" || len(info.Translators) > 0 || info.AgeRestriction != "" {
		args = append(args, "-metadata", "comment="+info.FormattedDescription())
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

func getBitrate(ctx context.Context, filePath string) (string, error) {
	cmd := exec.CommandContext(ctx, "ffprobe")
	cmd.Args = append(cmd.Args, "-v", "error", "-show_entries", "format=bit_rate", "-of", "default=noprint_wrappers=1:nokey=1", filePath)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ffprobe bitrate failed for %s: %w", filePath, err)
	}
	return string(out), nil
}

func needsM4AConversion(ctx context.Context, ext, m4aPath, srcPath string) bool {
	lowerExt := strings.ToLower(ext)
	if lowerExt == ".mp3" {
		if stat, err := os.Stat(m4aPath); err == nil && stat.Size() > 0 {
			return false
		}
		return true
	}

	if lowerExt == ".m4a" {
		bitrateStr, err := getBitrate(ctx, srcPath)
		if err != nil {
			return true
		}
		bitrate, err := strconv.Atoi(strings.TrimSpace(bitrateStr))
		if err != nil {
			return true
		}
		if bitrate > 0 && bitrate <= 66000 {
			return false
		}
	}
	return true
}

func processSingleAudioFile(ctx context.Context, targetDir, file string) error {
	srcPath := filepath.Join(targetDir, file)
	ext := filepath.Ext(file)
	base := strings.TrimSuffix(file, ext)
	m4aName := base + ".m4a"
	m4aPath := filepath.Join(targetDir, m4aName)

	if !needsM4AConversion(ctx, ext, m4aPath, srcPath) {
		return nil
	}

	tmpM4aPath := m4aPath + ".tmp.m4a"
	args := []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-i", srcPath,
		"-vn",
		"-c:a", "aac", "-b:a", "64k",
		tmpM4aPath,
	}
	cmd := exec.CommandContext(ctx, "ffmpeg")
	cmd.Args = append(cmd.Args, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to convert %s to m4a: %w\nOutput: %s", file, err, string(out))
	}
	if err := os.Rename(tmpM4aPath, m4aPath); err != nil {
		return fmt.Errorf("failed to rename temp file for %s: %w", file, err)
	}

	return nil
}

func convertAllToM4A(ctx context.Context, targetDir string, sourceFiles []string) error {
	totalFiles := len(sourceFiles)
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

	for _, file := range sourceFiles {
		g.Go(func() error {
			if err := processSingleAudioFile(gCtx, targetDir, file); err != nil {
				return err
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

func defaultWriteFile(name string, data []byte, perm os.FileMode) error {
	err := os.WriteFile(name, data, perm)
	if err != nil {
		return fmt.Errorf("write file error: %w", err)
	}
	return nil
}

var writeFile = defaultWriteFile
