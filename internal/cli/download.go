package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/underhax/audiobook-tools/internal/core"
	"github.com/underhax/audiobook-tools/pkg/utils"
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
	retries := fs.Int("retry", 3, "Maximum number of network retries")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse flags: %w", err)
	}

	if *url == "" {
		return errors.New("-url flag is required")
	}

	if *debug {
		if err := osSetenv("DEBUG", "1"); err != nil {
			return fmt.Errorf("set debug env: %w", err)
		}
	}

	d := newDownloader(*workers, *retries)

	var (
		info      *core.BookInfo
		chapters  []core.Chapter
		targetDir string
		err       error
	)

	for {
		info, chapters, targetDir, err = d.DownloadBook(context.Background(), *url, *outDir, *loadCover, *createMetadata, *detiVersion)
		if err != nil {
			if strings.Contains(err.Error(), "wait for chapters:") && len(unfinishedDownloads(targetDir)) > 0 {
				if askRetry(osStdin, out) {
					continue
				}
				return nil
			}
			return handleDownloadError(err, targetDir, out)
		}
		break
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
	absPath, absErr := filepathAbs(targetDir)
	if absErr != nil {
		return fmt.Errorf("get absolute path: %w", absErr)
	}
	targetDir = absPath

	start := time.Now()

	if err := m4bCheckDependencies(); err != nil {
		return fmt.Errorf("missing dependencies: %w", err)
	}
	if tmpFiles := unfinishedDownloads(targetDir); len(tmpFiles) > 0 {
		if err := utils.PrintUnfinishedWarning(stderrWriter, tmpFiles, "Please finish downloading before building M4B."); err != nil {
			return fmt.Errorf("print warning: %w", err)
		}
		return nil
	}
	if _, err := fmt.Fprintln(out, "Building M4B..."); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	outPath, err := m4bBuild(ctx, info, chapters, targetDir, debug)
	if err != nil {
		return fmt.Errorf("build m4b failed: %w", err)
	}

	if cleanFiles {
		if _, err := fmt.Fprintln(out, "Cleaning intermediate files..."); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		if err := m4bCleanIntermediateFiles(targetDir); err != nil {
			return fmt.Errorf("cleanup failed: %w", err)
		}
	}

	elapsed := time.Since(start)
	if _, err := fmt.Fprintf(out, "Build completed successfully in %s!\nOutput file: %s\n", elapsed.Round(time.Second), outPath); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func handleDownloadError(err error, targetDir string, out io.Writer) error {
	errMsg := err.Error()
	if authErr, ok := errors.AsType[*core.AuthError](err); ok {
		msg := fmt.Sprintf("Warning: Authentication required for %s.\n"+
			"Please provide your token using one of the following methods:\n"+
			"  1. Run: audiobook-tools auth %s <token>\n"+
			"  2. Env: %s=<token> audiobook-tools download ...",
			authErr.ProviderName, authErr.ProviderID, authErr.EnvVar)
		if _, wErr := fmt.Fprintln(stderrWriter, msg); wErr != nil {
			return fmt.Errorf("write output: %w", wErr)
		}
		return nil
	}

	if strings.HasPrefix(err.Error(), "prepare directory:") {
		msg := fmt.Sprintf("Skipping download: the destination path '%s' is invalid or not writable (%v)", targetDir, err)
		if _, wErr := fmt.Fprintln(stderrWriter, msg); wErr != nil {
			return fmt.Errorf("write output: %w", wErr)
		}
		return nil
	}

	isWarningErr := strings.Contains(err.Error(), "check your token or subscription") ||
		strings.Contains(err.Error(), "API error") ||
		strings.Contains(err.Error(), "paid books from knigavuhe") ||
		strings.Contains(err.Error(), "current subscription does not allow")

	if !isWarningErr {
		return fmt.Errorf("download failed: %w", err)
	}

	if idx := strings.Index(errMsg, "API error:"); idx != -1 {
		errMsg = errMsg[idx:]
	} else if idx := strings.Index(errMsg, "paid books"); idx != -1 {
		errMsg = errMsg[idx:]
	} else if idx := strings.Index(errMsg, "current subscription does not allow"); idx != -1 {
		errMsg = errMsg[idx:]
	}

	if _, wErr := fmt.Fprintf(stderrWriter, "Warning: %s\n", errMsg); wErr != nil {
		return fmt.Errorf("write output: %w", wErr)
	}
	if targetDir != "" {
		if _, wErr := fmt.Fprintf(out, "Basic metadata and cover were saved to: %s\n", targetDir); wErr != nil {
			return fmt.Errorf("write output: %w", wErr)
		}
	}
	return nil
}
