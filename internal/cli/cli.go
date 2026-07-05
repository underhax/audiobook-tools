// Package cli provides the command-line interface logic.
package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/underhax/audiobook-tools/internal/core"
	"github.com/underhax/audiobook-tools/internal/downloader"
	"github.com/underhax/audiobook-tools/internal/m4b"
)

// AppVersion is injected at build time by the GitHub Action.
var AppVersion = "dev"

// bookDownloader describes the downloader interface for DI.
type bookDownloader interface {
	DownloadBook(ctx context.Context, url, outputDir string, loadCover, createMetadata bool, version int) (*core.BookInfo, []core.Chapter, string, error)
}

// defaultDownloader is a wrapper to allow injecting mock downloaders in tests.
func defaultDownloader(workers int) bookDownloader {
	return downloader.New(workers)
}

var (
	m4bCheckDependencies      = m4b.CheckDependencies
	m4bExtractChaptersFromDir = m4b.ExtractChaptersFromDir
	m4bBuild                  = m4b.Build
	m4bCleanIntermediateFiles = m4b.CleanIntermediateFiles
	filepathAbs               = filepath.Abs

	newDownloader           = defaultDownloader
	stderrWriter  io.Writer = os.Stderr
	osSetenv                = os.Setenv
)
