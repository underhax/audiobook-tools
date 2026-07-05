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

	"github.com/underhax/audiobook-tools/internal/core"
	"github.com/underhax/audiobook-tools/internal/scrapers"
	"github.com/underhax/audiobook-tools/pkg/utils"
	"github.com/underhax/audiobook-tools/pkg/utils/httputil"
	"golang.org/x/sync/errgroup"
)

// Downloader provides configuration for concurrency and network timeouts during the retrieval process.
type Downloader struct {
	Client  *http.Client
	Workers int
}

// New initializes a Downloader with a constrained worker pool to prevent overwhelming target servers.
func New(workers int) *Downloader {
	if workers <= 0 {
		workers = 5
	}
	return &Downloader{
		Client: &http.Client{
			Transport: &httputil.RetryTransport{
				MaxRetries: 3,
			},
		},
		Workers: workers,
	}
}

// DownloadBook coordinates the complete retrieval pipeline: fetching metadata, resolving chapter URLs, and executing concurrent downloads.
func (d *Downloader) DownloadBook(ctx context.Context, url, outputDir string, loadCover, createMetadata bool, version int) (*core.BookInfo, []core.Chapter, string, error) {
	scraper, err := getScraper(url)
	if err != nil {
		return nil, nil, "", err
	}

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
		if errDir == nil {
			d.processExtras(ctx, &info, targetDir, loadCover, createMetadata)
		} else {
			log.Printf("Warning: could not create directory for metadata: %v\n", errDir)
		}
	}

	if errInfo != nil {
		return &info, nil, targetDir, fmt.Errorf("get book info: %w", errInfo)
	}

	if err := d.downloadChapters(ctx, chapters, targetDir); err != nil {
		return nil, nil, "", err
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
		return "", fmt.Errorf("create directory: %w", err)
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
		xmlStr, err := core.GenerateOPF(info)
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
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(d.Workers)

	for i, chapter := range chapters {
		eg.Go(func() error {
			ext := chapter.Extension
			if ext == "" {
				ext = ".mp3"
			}
			fileName := fmt.Sprintf("%03d %s%s", i+1, utils.SanitizeFilename(chapter.Title), ext)
			filePath := filepath.Join(targetDir, fileName)
			log.Printf("Downloading: %s\n", fileName)
			if err := d.downloadFile(egCtx, chapter.URL, filePath); err != nil {
				return fmt.Errorf("download %s: %w", fileName, err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("wait for chapters: %w", err)
	}
	return nil
}

func (d *Downloader) downloadFile(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("create file request: %w", err)
	}
	utils.SetHeaders(req)

	resp, err := d.Client.Do(req)
	if err != nil {
		return fmt.Errorf("do file request: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("failed to close file response body: %v", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	cleanPath := filepath.Clean(path)
	out, err := os.Create(cleanPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer func() {
		if cerr := out.Close(); cerr != nil {
			log.Printf("failed to close file: %v", cerr)
		}
	}()

	if _, err = io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("copy file content: %w", err)
	}
	return nil
}
