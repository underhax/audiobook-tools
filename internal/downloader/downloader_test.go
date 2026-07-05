package downloader

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/underhax/audiobook-tools/internal/core"
)

func TestDownloader_DownloadBook(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/book" {
			const tmpl = `
			<html>
			<body>
				<div class="page_title">
					<span itemprop="name">Test Book</span>
					<span itemprop="author">Test Author</span>
				</div>
				<script>
					var player = new BookPlayer([{"url":"http://{{.}}/1.mp3","title":"Ch 1"}]);
				</script>
			</body>
			</html>
			`
			w.Header().Set("Content-Type", "text/plain")
			tmplParsed := template.Must(template.New("html").Parse(tmpl))
			if err := tmplParsed.Execute(w, r.Host); err != nil {
				return
			}
			return
		}
		if r.URL.Path == "/1.mp3" {
			w.Header().Set("Content-Type", "text/plain")
			if _, err := w.Write([]byte("audio content")); err != nil {
				return
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	d := New(0)
	d.Client = srv.Client()

	outDir := t.TempDir()
	bookURL := srv.URL + "/book?knigavuhe.org"

	_, _, _, err := d.DownloadBook(context.Background(), bookURL, outDir, false, true, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	targetDir := filepath.Join(outDir, "Test Author", "Test Book")
	mp3Path := filepath.Join(targetDir, "001 Ch 1.mp3")
	opfPath := filepath.Join(targetDir, "metadata.opf")

	if _, err := os.Stat(mp3Path); os.IsNotExist(err) {
		t.Errorf("MP3 file not found: %s", mp3Path)
	}
	if _, err := os.Stat(opfPath); os.IsNotExist(err) {
		t.Errorf("OPF file not found: %s", opfPath)
	}
}

func TestDownloader_Errors(t *testing.T) {
	d := New(2)

	_, _, _, err := d.DownloadBook(context.Background(), "http://%invalid", t.TempDir(), false, false, 1)
	if err == nil {
		t.Error("expected error for bad url")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d.Client = srv.Client()
	_, _, _, err = d.DownloadBook(context.Background(), srv.URL+"/book", t.TempDir(), false, false, 1)
	if err == nil {
		t.Error("expected error for unsupported scraper")
	}

	srv404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv404.Close()
	d.Client = srv404.Client()
	_, _, _, err = d.DownloadBook(context.Background(), srv404.URL+"/book?knigavuhe.org", t.TempDir(), false, false, 1)
	if err == nil {
		t.Error("expected error for 404")
	}

	_, _, _, err = d.DownloadBook(context.Background(), "https://knigavuhe.org/paid/book/something", t.TempDir(), false, false, 1)
	if err == nil || err.Error() != "paid books from knigavuhe.org are not supported" {
		t.Errorf("expected paid book error, got: %v", err)
	}

	tempFile := filepath.Join(t.TempDir(), "file.txt")
	if writeErr := os.WriteFile(tempFile, []byte("test"), 0o600); writeErr == nil {
		var srvURL string
		srvOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/mp3" {
				w.WriteHeader(http.StatusOK)
				return
			}
			tmpl := fmt.Sprintf(`<html><body><span itemprop="name">Title</span><script>var player = new BookPlayer([{"title":"Ch1","url":"%s/mp3"}]);</script></body></html>`, srvURL)
			if _, wErr := w.Write([]byte(tmpl)); wErr != nil {
				return
			}
		}))
		srvURL = srvOK.URL
		defer srvOK.Close()
		d.Client = srvOK.Client()
		_, _, _, err = d.DownloadBook(context.Background(), srvOK.URL+"/book?knigavuhe.org", tempFile, false, false, 1)
		if err == nil || !strings.HasPrefix(err.Error(), "prepare directory:") {
			t.Errorf("expected prepare directory error, got: %v", err)
		}
	}

	srvBadJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		const html = `<html><body><div class="book_title">Title</div><script>var player = new BookPlayer([{bad JSON}]);</script></body></html>`
		if _, wErr := w.Write([]byte(html)); wErr != nil {
			return
		}
	}))
	defer srvBadJSON.Close()
	d.Client = srvBadJSON.Client()
	_, _, _, err = d.DownloadBook(context.Background(), srvBadJSON.URL+"/book?knigavuhe.org", t.TempDir(), false, false, 1)
	if err == nil {
		t.Error("expected error for missing bookData")
	}
}

func TestDownloader_Errors_Scrapers(t *testing.T) {
	d := New(2)

	srvDeti := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srvDeti.Close()
	d.Client = srvDeti.Client()
	_, _, _, err := d.DownloadBook(context.Background(), srvDeti.URL+"/book?deti-online.com", t.TempDir(), false, false, 2)
	if err != nil {
		t.Logf("expected deti-online empty response: %v", err)
	}

	srvBadMP3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		const html = `<html><body><div class="book_title">Title</div><script>var player = new BookPlayer([{"title":"Ch1","url":"http://%invalid/mp3"}]);</script></body></html>`
		if _, wErr := w.Write([]byte(html)); wErr != nil {
			return
		}
	}))
	defer srvBadMP3.Close()
	d.Client = srvBadMP3.Client()
	_, _, _, err = d.DownloadBook(context.Background(), srvBadMP3.URL+"/book?knigavuhe.org", t.TempDir(), false, false, 1)
	if err == nil || !strings.HasPrefix(err.Error(), "wait for chapters: download 001 Ch1.mp3:") {
		t.Errorf("expected downloadFile error, got: %v", err)
	}
}

func TestDownloader_downloadFileError(t *testing.T) {
	d := New(1)
	err := d.downloadFile(context.Background(), "http://%invalid", "out.mp3")
	if err == nil {
		t.Error("expected error")
	}
}

type mockBody struct {
	readErr  error
	closeErr error
}

func (m *mockBody) Read(_ []byte) (n int, err error) {
	if m.readErr != nil {
		return 0, m.readErr
	}
	return 0, io.EOF
}

func (m *mockBody) Close() error {
	return m.closeErr
}

type mockTransport struct {
	resp *http.Response
	err  error
}

func (m *mockTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return m.resp, m.err
}

func TestDownloader_fetchHTML_Errors(t *testing.T) {
	d := New(1)

	_, err := d.fetchHTML(context.Background(), "http://%invalid")
	if err == nil || !strings.Contains(err.Error(), "create request:") {
		t.Errorf("expected create request error, got: %v", err)
	}

	d.Client = &http.Client{
		Transport: &mockTransport{err: errors.New("mock do error")},
	}
	_, err = d.fetchHTML(context.Background(), "http://example.com")
	if err == nil || !strings.Contains(err.Error(), "fetch page:") {
		t.Errorf("expected fetch page error, got: %v", err)
	}

	d.Client = &http.Client{
		Transport: &mockTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       &mockBody{readErr: errors.New("mock read error")},
			},
		},
	}
	_, err = d.fetchHTML(context.Background(), "http://example.com")
	if err == nil || !strings.Contains(err.Error(), "read body:") {
		t.Errorf("expected read body error, got: %v", err)
	}

	d.Client = &http.Client{
		Transport: &mockTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       &mockBody{readErr: io.EOF, closeErr: errors.New("mock close error")},
			},
		},
	}
	_, err = d.fetchHTML(context.Background(), "http://example.com")
	if err != nil {
		t.Errorf("expected no error for close failure, got: %v", err)
	}
}

func TestGetScraper(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"knigavuhe", "http://knigavuhe.org/book/1", false},
		{"deti-online", "http://deti-online.com/book/1", false},
		{"yandex", "https://books.yandex.ru/book/1", false},
		{"paid", "http://knigavuhe.org/paid/book/1", true},
		{"unsupported", "http://example.com/book/1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scraper, err := getScraper(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if scraper == nil {
					t.Errorf("expected scraper, got nil")
				}
			}
		})
	}
}

func TestDownloader_prepareDirectory(t *testing.T) {
	d := New(0)
	outDir := t.TempDir()

	tests := []struct {
		name     string
		expected string
		info     core.BookInfo
		version  int
	}{
		{
			name: "deti-online version > 1",
			info: core.BookInfo{
				URL:    "http://deti-online.com/book",
				Author: "AuthorDeti",
				Title:  "TitleDeti",
			},
			version:  2,
			expected: "TitleDeti (Version 2)",
		},
		{
			name: "knigavuhe with narrator",
			info: core.BookInfo{
				URL:      "http://knigavuhe.org/book",
				Author:   "AuthorKniga",
				Title:    "TitleKniga",
				Narrator: "Narrator",
			},
			version:  1,
			expected: "TitleKniga (Narrator)",
		},
		{
			name: "normal book",
			info: core.BookInfo{
				URL:    "http://example.com/book",
				Author: "AuthorNormal",
				Title:  "TitleNormal",
			},
			version:  1,
			expected: "TitleNormal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDir, err := d.prepareDirectory(&tt.info, outDir, tt.version)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			expectedDir := filepath.Join(outDir, tt.info.Author, tt.expected)
			if gotDir != expectedDir {
				t.Errorf("expected directory %s, got %s", expectedDir, gotDir)
			}
		})
	}
}

func TestDownloader_processExtras(t *testing.T) {
	d := New(0)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cover.jpg" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	d.Client = srv.Client()

	tests := []struct {
		name           string
		coverURL       string
		mockDir        string
		checkFile      string
		createMetadata bool
		loadCover      bool
		expectExist    bool
	}{
		{
			name:           "success cover",
			coverURL:       srv.URL + "/cover.jpg",
			loadCover:      true,
			createMetadata: false,
			checkFile:      "cover.jpg",
			expectExist:    true,
		},
		{
			name:           "cover error",
			coverURL:       srv.URL + "/404.jpg",
			loadCover:      true,
			createMetadata: false,
			checkFile:      "cover.jpg",
			expectExist:    false,
		},
		{
			name:           "write metadata error",
			coverURL:       "",
			loadCover:      false,
			createMetadata: true,
			mockDir:        "metadata.opf",
			checkFile:      "metadata.opf",
			expectExist:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outDir := t.TempDir()
			if tt.mockDir != "" {
				if err := os.Mkdir(filepath.Join(outDir, tt.mockDir), 0o750); err != nil {
					t.Fatalf("mkdir error: %v", err)
				}
			}

			info := &core.BookInfo{Cover: tt.coverURL, Title: "TitleExtras"}
			d.processExtras(context.Background(), info, outDir, tt.loadCover, tt.createMetadata)

			path := filepath.Join(outDir, tt.checkFile)
			infoStat, err := os.Stat(path)
			exists := !os.IsNotExist(err)
			if exists != tt.expectExist {
				t.Errorf("expected exists %v, got %v", tt.expectExist, exists)
			}
			if tt.mockDir != "" && exists && !infoStat.IsDir() {
				t.Errorf("expected %s to remain a directory", tt.checkFile)
			}
		})
	}
}

func mockGenerateOPFError(_ *core.BookInfo) (string, error) {
	return "", errors.New("mock opf error")
}

func TestDownloader_processExtras_GenerateOPFError(t *testing.T) {
	origGenerate := generateOPF
	generateOPF = mockGenerateOPFError
	defer func() { generateOPF = origGenerate }()

	d := New(0)
	outDir := t.TempDir()
	info := &core.BookInfo{Title: "Test OPF Error"}

	d.processExtras(context.Background(), info, outDir, false, true)

	if _, err := os.Stat(filepath.Join(outDir, "metadata.opf")); !os.IsNotExist(err) {
		t.Error("expected metadata.opf to not be created")
	}
}

func TestDownloader_downloadFile_AllErrors(t *testing.T) {
	d := New(1)
	outDir := t.TempDir()

	tests := []struct {
		setup      func(*Downloader, string) string
		name       string
		errMessage string
		wantErr    bool
	}{
		{
			name: "client do error",
			setup: func(d *Downloader, dir string) string {
				d.Client = &http.Client{
					Transport: &mockTransport{err: errors.New("mock do error")},
				}
				return filepath.Join(dir, "test1.mp3")
			},
			wantErr:    true,
			errMessage: "do file request:",
		},
		{
			name: "body close error",
			setup: func(d *Downloader, dir string) string {
				d.Client = &http.Client{
					Transport: &mockTransport{
						resp: &http.Response{
							StatusCode: http.StatusOK,
							Body:       &mockBody{closeErr: errors.New("mock close error")},
						},
					},
				}
				return filepath.Join(dir, "test2.mp3")
			},
			wantErr: false,
		},
		{
			name: "create file error",
			setup: func(d *Downloader, dir string) string {
				d.Client = &http.Client{
					Transport: &mockTransport{
						resp: &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader("data")),
						},
					},
				}
				return dir
			},
			wantErr:    true,
			errMessage: "create file:",
		},
		{
			name: "copy content error",
			setup: func(d *Downloader, dir string) string {
				d.Client = &http.Client{
					Transport: &mockTransport{
						resp: &http.Response{
							StatusCode: http.StatusOK,
							Body:       &mockBody{readErr: errors.New("mock read error")},
						},
					},
				}
				return filepath.Join(dir, "test3.mp3")
			},
			wantErr:    true,
			errMessage: "copy file content:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(d, outDir)
			err := d.downloadFile(context.Background(), "http://example.com/file", path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errMessage)
				}
				if !strings.Contains(err.Error(), tt.errMessage) {
					t.Errorf("expected error containing %q, got %v", tt.errMessage, err)
				}
			} else if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

type mockWriteCloser struct {
	io.Writer
	closeErr error
}

func (m *mockWriteCloser) Close() error {
	return m.closeErr
}

func mockCreateFileErrorClose(_ string) (io.WriteCloser, error) {
	return &mockWriteCloser{
		Writer:   io.Discard,
		closeErr: errors.New("mock file close error"),
	}, nil
}

func TestDownloader_downloadFile_CloseError(t *testing.T) {
	origCreateFile := createFile
	createFile = mockCreateFileErrorClose
	defer func() { createFile = origCreateFile }()

	d := New(1)

	d.Client = &http.Client{
		Transport: &mockTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("data")),
			},
		},
	}

	err := d.downloadFile(context.Background(), "http://example.com/file", "dummy.mp3")
	if err != nil {
		t.Errorf("expected no error from downloadFile since close error is only logged, got %v", err)
	}
}
