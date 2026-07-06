// Package cli provides the command-line interface logic.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/underhax/audiobook-tools/internal/core"
	"github.com/underhax/audiobook-tools/internal/downloader"
	"github.com/underhax/audiobook-tools/internal/m4b"
)

// AppVersion is injected at build time by the GitHub Action.
var AppVersion = "dev"

type bookDownloader interface {
	DownloadBook(ctx context.Context, url, outputDir string, loadCover, createMetadata bool, version int) (*core.BookInfo, []core.Chapter, string, error)
}

func defaultDownloader(workers, retries int) bookDownloader {
	return downloader.New(workers, retries)
}

var (
	m4bCheckDependencies      = m4b.CheckDependencies
	m4bExtractChaptersFromDir = m4b.ExtractChaptersFromDir
	m4bBuild                  = m4b.Build
	m4bCleanIntermediateFiles = m4b.CleanIntermediateFiles
	filepathAbs               = filepath.Abs

	newDownloader           = defaultDownloader
	stderrWriter  io.Writer = os.Stderr
	osStdin       io.Reader = os.Stdin
	osSetenv                = os.Setenv
	osReadDir               = os.ReadDir
)

func unfinishedDownloads(dir string) []string {
	entries, err := osReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".tmp" {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files
}

func askRetry(in io.Reader, out io.Writer) bool {
	if _, err := io.WriteString(out, "Do you want to retry downloading the missing files? [yes/No]: "); err != nil {
		return false
	}

	var response string
	if _, err := fmt.Fscanln(in, &response); err != nil {
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
