package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/underhax/audiobook-tools/internal/core"
	"github.com/underhax/audiobook-tools/internal/m4b"
)

// RunBuild parses flags and executes the independent M4B build workflow.
func RunBuild(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(out)

	dir := fs.String("dir", "", "Path to the directory containing the audiobook files")
	cleanFiles := fs.Bool("clean", false, "Clean up downloaded MP3 files after building M4B")
	debug := fs.Bool("debug", false, "Show ffmpeg output and warnings")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse flags: %w", err)
	}

	if *dir == "" {
		return errors.New("-dir flag is required")
	}

	if err := m4b.CheckDependencies(); err != nil {
		return fmt.Errorf("missing dependencies: %w", err)
	}

	info := getMetadata(*dir)

	chapters, err := m4b.ExtractChaptersFromDir(*dir)
	if err != nil {
		return fmt.Errorf("failed to extract chapters: %w", err)
	}

	ctx := context.Background()
	return executeBuild(ctx, info, chapters, dir, cleanFiles, debug, out)
}

func executeBuild(ctx context.Context, info *core.BookInfo, chapters []core.Chapter, dir *string, cleanFiles, debug *bool, out io.Writer) error {
	start := time.Now()

	if _, err := fmt.Fprintln(out, "Building M4B..."); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	outPath, err := m4b.Build(ctx, info, chapters, *dir, *debug)
	if err != nil {
		return fmt.Errorf("build m4b failed: %w", err)
	}

	if *cleanFiles {
		if _, err := fmt.Fprintln(out, "Cleaning intermediate files..."); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		if err := m4b.CleanIntermediateFiles(*dir); err != nil {
			return fmt.Errorf("cleanup failed: %w", err)
		}
	}

	elapsed := time.Since(start)
	if _, err := fmt.Fprintf(out, "Build completed successfully in %s!\nOutput file: %s\n", elapsed.Round(time.Second), outPath); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

const (
	unknownBook   = "Unknown Book"
	unknownAuthor = "Unknown Author"
)

func getMetadata(dir string) *core.BookInfo {
	info, err := core.ParseOPF(filepath.Join(dir, "metadata.opf"))
	if err == nil && info != nil && info.Title != "" {
		return info
	}

	if info := getMetadataFromID3(dir); info != nil {
		return info
	}

	return getMetadataFromPath(dir)
}

func getMetadataFromID3(dir string) *core.BookInfo {
	mp3s, err := filepath.Glob(filepath.Join(dir, "*.mp3"))
	if err != nil || len(mp3s) == 0 {
		return nil
	}

	firstMP3 := mp3s[0]
	title := m4b.ExtractID3Text(firstMP3, "TALB")
	if title == "" {
		title = m4b.ExtractID3Text(firstMP3, "TIT2")
	}
	author := m4b.ExtractID3Text(firstMP3, "TPE1")
	narrator := m4b.ExtractID3Text(firstMP3, "TPE2")

	if title != "" || author != "" {
		if title == "" {
			title = unknownBook
		}
		if author == "" {
			author = unknownAuthor
		}
		return &core.BookInfo{
			Title:    title,
			Author:   author,
			Narrator: narrator,
		}
	}
	return nil
}

func getMetadataFromPath(dir string) *core.BookInfo {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return &core.BookInfo{Title: unknownBook, Author: unknownAuthor}
	}

	title := filepath.Base(absDir)
	author := filepath.Base(filepath.Dir(absDir))
	if author == "." || author == "/" {
		author = unknownAuthor
	}
	if title == "" || title == "." || title == "/" {
		title = unknownBook
	}

	return &core.BookInfo{
		Title:  title,
		Author: author,
	}
}
