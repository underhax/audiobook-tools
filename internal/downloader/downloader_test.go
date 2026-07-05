package downloader

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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
}

func TestDownloader_downloadFileError(t *testing.T) {
	d := New(1)
	err := d.downloadFile(context.Background(), "http://%invalid", "out.mp3")
	if err == nil {
		t.Error("expected error")
	}
}
