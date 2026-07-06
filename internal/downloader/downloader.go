// Package downloader coordinates the retrieval and storage of audiobook files.
package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/underhax/audiobook-tools/internal/core"
	"github.com/underhax/audiobook-tools/internal/scrapers"
	"github.com/underhax/audiobook-tools/pkg/utils"
	"github.com/underhax/audiobook-tools/pkg/utils/httputil"
	"github.com/underhax/audiobook-tools/pkg/utils/spinner"
	"golang.org/x/sync/errgroup"
)

var generateOPF = core.GenerateOPF

func defaultOpenFile(name string, flag int, perm os.FileMode) (io.WriteCloser, error) {
	out, err := os.OpenFile(filepath.Clean(name), flag, perm)
	if err != nil {
		return nil, fmt.Errorf("os openfile: %w", err)
	}
	return out, nil
}

func defaultRenameFile(oldpath, newpath string) error {
	if err := os.Rename(oldpath, newpath); err != nil {
		return fmt.Errorf("os rename: %w", err)
	}
	return nil
}

func defaultStatFile(name string) (os.FileInfo, error) {
	fi, err := os.Stat(name)
	if err != nil {
		return nil, fmt.Errorf("os stat: %w", err)
	}
	return fi, nil
}

var (
	openFile   = defaultOpenFile
	renameFile = defaultRenameFile
	statFile   = defaultStatFile
	sleepFunc  = defaultSleepFunc
)

func defaultSleepFunc(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("context done: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// Downloader provides configuration for concurrency and network timeouts during the retrieval process.
type Downloader struct {
	Client     *http.Client
	Workers    int
	MaxRetries int
}

// New initializes a Downloader with a constrained worker pool to prevent overwhelming target servers.
func New(workers, retries int) *Downloader {
	if workers <= 0 {
		workers = 5
	}
	return &Downloader{
		Client: &http.Client{
			Transport: httputil.NewRetryTransport(httputil.WithMaxRetries(retries)),
		},
		Workers:    workers,
		MaxRetries: retries,
	}
}

// DownloadBook coordinates the complete retrieval pipeline: fetching metadata, resolving chapter URLs, and executing concurrent downloads.
func (d *Downloader) DownloadBook(ctx context.Context, url, outputDir string, loadCover, createMetadata bool, version int) (*core.BookInfo, []core.Chapter, string, error) {
	scraper, err := getScraper(url)
	if err != nil {
		return nil, nil, "", err
	}
	scraper.SetClient(d.Client)

	var htmlContent string
	if !strings.Contains(url, "books.yandex.ru") {
		htmlContent, err = d.fetchHTML(ctx, url)
		if err != nil {
			return nil, nil, "", err
		}
	}

	if doScraper, ok := scraper.(*scrapers.DetiOnline); ok {
		doScraper.Version = version
	}

	info, chapters, errInfo := scraper.GetBookInfo(ctx, htmlContent, url)

	var targetDir string
	if info.Title != "" {
		log.Printf("Found book: %s by %s\n", info.Title, info.Author)
		var errDir error
		targetDir, errDir = d.prepareDirectory(&info, outputDir, version)
		if errDir != nil {
			return nil, nil, targetDir, fmt.Errorf("prepare directory: %w", errDir)
		}
		d.processExtras(ctx, &info, targetDir, loadCover, createMetadata)
	}

	if errInfo != nil {
		return &info, nil, targetDir, fmt.Errorf("get book info: %w", errInfo)
	}

	if err := d.downloadChapters(ctx, chapters, targetDir); err != nil {
		return nil, nil, targetDir, err
	}

	return &info, chapters, targetDir, nil
}

func (d *Downloader) fetchHTML(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	utils.SetHeaders(req)

	resp, err := d.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch page: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("failed to close response body: %v", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	htmlContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	return string(htmlContent), nil
}

func getScraper(bookURL string) (scrapers.Scraper, error) {
	switch {
	case strings.Contains(bookURL, "knigavuhe.org"):
		if strings.Contains(bookURL, "/paid/book/") {
			return nil, errors.New("paid books from knigavuhe.org are not supported")
		}
		return scrapers.NewKnigavuhe(), nil
	case strings.Contains(bookURL, "deti-online.com"):
		return scrapers.NewDetiOnline(), nil
	case strings.Contains(bookURL, "books.yandex.ru"):
		return scrapers.NewBooksYandex(), nil
	default:
		return nil, fmt.Errorf("unsupported website: %s", bookURL)
	}
}

func (d *Downloader) prepareDirectory(info *core.BookInfo, outputDir string, version int) (string, error) {
	authorFolder := utils.SanitizeFilename(info.Author)
	bookName := info.Title
	if version > 1 && strings.Contains(info.URL, "deti-online.com") {
		bookName = fmt.Sprintf("%s (Version %d)", bookName, version)
	} else if strings.Contains(info.URL, "knigavuhe.org") && info.Narrator != "" {
		bookName = fmt.Sprintf("%s (%s)", bookName, info.Narrator)
	}
	bookFolder := utils.SanitizeFilename(bookName)
	targetDir := filepath.Join(outputDir, authorFolder, bookFolder)

	const dirPerm = 0o755
	if err := os.MkdirAll(targetDir, dirPerm); err != nil {
		return targetDir, fmt.Errorf("create directory: %w", err)
	}
	return targetDir, nil
}

func (d *Downloader) processExtras(ctx context.Context, info *core.BookInfo, targetDir string, loadCover, createMetadata bool) {
	if loadCover && info.Cover != "" {
		if err := d.downloadFile(ctx, info.Cover, filepath.Join(targetDir, "cover.jpg")); err != nil {
			log.Printf("Failed to download cover: %v", err)
		}
	}

	if createMetadata {
		xmlStr, err := generateOPF(info)
		if err != nil {
			log.Printf("Failed to generate metadata: %v", err)
		} else {
			opfPath := filepath.Join(targetDir, "metadata.opf")
			const filePerm = 0o600
			if err := os.WriteFile(opfPath, []byte(xmlStr), filePerm); err != nil {
				log.Printf("Failed to write metadata: %v", err)
			}
		}
	}
}

func (d *Downloader) downloadChapters(ctx context.Context, chapters []core.Chapter, targetDir string) error {
	var eg errgroup.Group
	eg.SetLimit(d.Workers)
	total := len(chapters)
	var completed atomic.Int32

	var mu sync.Mutex
	var failedFiles []string

	stopSpinner := spinner.Start(ctx, "Downloading...", &completed, total)

	for i, chapter := range chapters {
		eg.Go(func() error {
			ext := chapter.Extension
			if ext == "" {
				ext = ".mp3"
			}
			fileName := fmt.Sprintf("%03d %s%s", i+1, utils.SanitizeFilename(chapter.Title), ext)
			filePath := filepath.Join(targetDir, fileName)

			if err := d.downloadFile(ctx, chapter.URL, filePath); err != nil {
				mu.Lock()
				failedFiles = append(failedFiles, fileName)
				mu.Unlock()
				return fmt.Errorf("download %s: %w", fileName, err)
			}
			completed.Add(1)
			return nil
		})
	}

	err := eg.Wait()
	stopSpinner()

	if err != nil {
		if len(failedFiles) > 0 {
			paths := make([]string, len(failedFiles))
			for i, f := range failedFiles {
				paths[i] = filepath.Join(targetDir, f+".tmp")
			}
			if wErr := utils.PrintUnfinishedWarning(os.Stderr, paths, ""); wErr != nil {
				return fmt.Errorf("print warning: %w", wErr)
			}
		}
		return fmt.Errorf("wait for chapters: %w", err)
	}

	return nil
}

func (d *Downloader) downloadFile(ctx context.Context, url, path string) error {
	cleanPath := filepath.Clean(path)

	if fi, err := statFile(cleanPath); err == nil && !fi.IsDir() {
		return nil
	}

	tmpPath := cleanPath + ".tmp"
	var lastErr error

	for attempt := 0; attempt <= d.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := httputil.CalculateBackoff(httputil.ActionRetry, attempt, 2*time.Second)
			if err := sleepFunc(ctx, delay); err != nil {
				return err
			}
		}

		err := d.doDownloadAttempt(ctx, url, tmpPath)
		if err == nil {
			if rerr := renameFile(tmpPath, cleanPath); rerr != nil {
				return fmt.Errorf("rename tmp file: %w", rerr)
			}
			return nil
		}

		lastErr = err
	}

	return fmt.Errorf("failed after %d attempts: %w", d.MaxRetries+1, lastErr)
}

func (d *Downloader) doDownloadAttempt(ctx context.Context, url, tmpPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("create file request: %w", err)
	}
	utils.SetHeaders(req)

	var resumeSize int64
	fi, statErr := statFile(tmpPath)
	if statErr == nil && !fi.IsDir() && fi.Size() > 0 {
		resumeSize = fi.Size()
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeSize))
	}

	resp, err := d.Client.Do(req)
	if err != nil {
		return fmt.Errorf("do file request: %w", err)
	}
	defer func() {
		cerr := resp.Body.Close()
		_ = cerr
	}()

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	openFlags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if resp.StatusCode == http.StatusPartialContent && resumeSize > 0 {
		openFlags = os.O_APPEND | os.O_WRONLY
	}

	out, err := openFile(tmpPath, openFlags, 0o644)
	if err != nil {
		return fmt.Errorf("open tmp file: %w", err)
	}

	var copyErr error
	if _, err = io.Copy(out, resp.Body); err != nil {
		copyErr = fmt.Errorf("copy file content: %w", err)
	}

	if cerr := out.Close(); cerr != nil {
		if copyErr == nil {
			copyErr = fmt.Errorf("close tmp file: %w", cerr)
		}
	}

	return copyErr
}
