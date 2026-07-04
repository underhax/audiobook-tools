package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/underhax/audiobook-tools/internal/core"
	"github.com/underhax/audiobook-tools/internal/downloader"
	"github.com/underhax/audiobook-tools/internal/m4b"
)

// RunDownload parses flags and executes the downloader workflow.
func RunDownload(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fs.SetOutput(out)

	url := fs.String("url", "", "URL of the audiobook to download")
	outDir := fs.String("out", ".", "Output directory for the downloaded files")
	workers := fs.Int("workers", 5, "Number of concurrent download workers")
	loadCover := fs.Bool("cover", true, "Download cover image")
	createMetadata := fs.Bool("metadata", true, "Create OPF metadata file")
	m4bFlag := fs.Bool("m4b", false, "Build M4B file after downloading")
	cleanFiles := fs.Bool("clean", false, "Clean up downloaded MP3 files after building M4B (only if -m4b is set)")
	debug := fs.Bool("debug", false, "Show ffmpeg output and warnings")
	detiVersion := fs.Int("deti-online-voice-version", 1, "Voice version to download (deti-online.com only)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse flags: %w", err)
	}

	if *url == "" {
		return errors.New("-url flag is required")
	}

	d := downloader.New(*workers)
	info, chapters, targetDir, err := d.DownloadBook(context.Background(), *url, *outDir, *loadCover, *createMetadata, *detiVersion)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	if _, err := fmt.Fprintln(out, "Download completed successfully."); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	if *m4bFlag {
		if err := executeBuilder(context.Background(), info, chapters, targetDir, *cleanFiles, *debug, out); err != nil {
			return fmt.Errorf("builder execution failed: %w", err)
		}
	}

	return nil
}

func executeBuilder(ctx context.Context, info *core.BookInfo, chapters []core.Chapter, targetDir string, cleanFiles, debug bool, out io.Writer) error {
	start := time.Now()

	if err := m4b.CheckDependencies(); err != nil {
		return fmt.Errorf("missing dependencies: %w", err)
	}
	if _, err := fmt.Fprintln(out, "Building M4B..."); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	outPath, err := m4b.Build(ctx, info, chapters, targetDir, debug)
	if err != nil {
		return fmt.Errorf("build m4b failed: %w", err)
	}

	if cleanFiles {
		if _, err := fmt.Fprintln(out, "Cleaning intermediate files..."); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		if err := m4b.CleanIntermediateFiles(targetDir); err != nil {
			return fmt.Errorf("cleanup failed: %w", err)
		}
	}

	elapsed := time.Since(start)
	if _, err := fmt.Fprintf(out, "Build completed successfully in %s!\nOutput file: %s\n", elapsed.Round(time.Second), outPath); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
