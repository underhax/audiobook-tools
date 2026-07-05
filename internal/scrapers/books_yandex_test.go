package scrapers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractUUID(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{"Valid standard URL", "https://example.com/audiobooks/xyz123", "xyz123", false},
		{"Valid URL with query", "https://example.com/audiobooks/abc456?foo=bar", "abc456", false},
		{"Invalid URL", "https://example.com/books/xyz123", "", true},
		{"Empty URL", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractUUID(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractUUID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractUUID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetToken_Env(t *testing.T) {
	t.Setenv("BOOKS_YANDEX_TOKEN", "test-token")
	token, err := GetToken()
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if token != "test-token" {
		t.Errorf("GetToken() = %v, want test-token", token)
	}
}

func TestGetToken_File(t *testing.T) {
	t.Setenv("BOOKS_YANDEX_TOKEN", "")

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("APPDATA", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	_, err := SaveToken("file-token")
	if err != nil {
		t.Fatalf("SaveToken() error = %v", err)
	}

	token, err := GetToken()
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if token != "file-token" {
		t.Errorf("GetToken() = %v, want file-token", token)
	}
}

func TestGetToken_Missing(t *testing.T) {
	t.Setenv("BOOKS_YANDEX_TOKEN", "")

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("APPDATA", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	_, err := GetToken()
	if err == nil {
		t.Fatal("GetToken() expected error for missing token")
	}
}

func TestBooksYandex_GetBookInfo(t *testing.T) {
	t.Setenv("BOOKS_YANDEX_TOKEN", "dummy-token")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v5/audiobooks/testuuid", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("auth-token") != "dummy-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{
			"audiobook": {
				"title": "Test Book",
				"can_be_listened": true,
				"authors": [{"name": "Test Author"}],
				"cover": {"large": "https://example.com/cover.jpg"}
			}
		}`)); err != nil {
			t.Errorf("w.Write error: %v", err)
		}
	})
	mux.HandleFunc("/api/v5/audiobooks/testuuid/playlists.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("auth-token") != "dummy-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{
			"tracks": [
				{"number": 0, "offline": {"max_bit_rate": {"url": "https://example.com/track1.m3u8"}}},
				{"number": 1, "offline": {"max_bit_rate": {"url": "https://example.com/track2.m3u8"}}}
			]
		}`)); err != nil {
			t.Errorf("w.Write error: %v", err)
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	scraper := NewBooksYandex()
	scraper.BaseURL = server.URL

	info, chapters, err := scraper.GetBookInfo(context.Background(), "", "https://example.com/audiobooks/testuuid")
	if err != nil {
		t.Fatalf("GetBookInfo() error = %v", err)
	}

	if info.Title != "Test Book" {
		t.Errorf("Expected title 'Test Book', got '%s'", info.Title)
	}
	if info.Author != "Test Author" {
		t.Errorf("Expected author 'Test Author', got '%s'", info.Author)
	}
	if len(chapters) != 2 {
		t.Fatalf("Expected 2 chapters, got %d", len(chapters))
	}
	if chapters[0].URL != "https://example.com/track1.m4a" {
		t.Errorf("Expected m4a URL, got '%s'", chapters[0].URL)
	}
	if chapters[0].Extension != ".m4a" {
		t.Errorf("Expected extension .m4a, got '%s'", chapters[0].Extension)
	}
}

func TestBooksYandex_GetBookInfo_MissingToken(t *testing.T) {
	t.Setenv("BOOKS_YANDEX_TOKEN", "")
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("APPDATA", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	scraper := NewBooksYandex()
	_, _, err := scraper.GetBookInfo(context.Background(), "", "https://example.com/audiobooks/testuuid")
	if err == nil {
		t.Fatal("GetBookInfo() expected error for missing token")
	}
}

func TestBooksYandex_GetBookInfo_InvalidURL(t *testing.T) {
	scraper := NewBooksYandex()
	_, _, err := scraper.GetBookInfo(context.Background(), "", "https://example.com/invalid/testuuid")
	if err == nil {
		t.Fatal("GetBookInfo() expected error for invalid URL")
	}
}

func TestBooksYandex_fetchInfo_Errors(t *testing.T) {
	scraper := NewBooksYandex()
	scraper.Client.Transport = http.DefaultTransport
	scraper.BaseURL = "http://localhost:0"

	_, err := scraper.fetchInfo(context.Background(), "uuid", "token")
	if err == nil {
		t.Fatal("fetchInfo() expected error")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v5/audiobooks/badjson", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, errWrite := w.Write([]byte(`{bad json`)); errWrite != nil {
			t.Errorf("w.Write error: %v", errWrite)
		}
	})
	mux.HandleFunc("/api/v5/audiobooks/badstatus", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	scraper.BaseURL = server.URL
	_, err = scraper.fetchInfo(context.Background(), "badjson", "token")
	if err == nil {
		t.Fatal("fetchInfo() expected json decode error")
	}
	_, err = scraper.fetchInfo(context.Background(), "badstatus", "token")
	if err == nil {
		t.Fatal("fetchInfo() expected status error")
	}
}

func TestBooksYandex_fetchPlaylists_Errors(t *testing.T) {
	scraper := NewBooksYandex()
	scraper.Client.Transport = http.DefaultTransport
	scraper.BaseURL = "http://localhost:0"

	_, err := scraper.fetchPlaylists(context.Background(), "uuid", "token")
	if err == nil {
		t.Fatal("fetchPlaylists() expected error")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v5/audiobooks/badjson/playlists.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, errWrite := w.Write([]byte(`{bad json`)); errWrite != nil {
			t.Errorf("w.Write error: %v", errWrite)
		}
	})
	mux.HandleFunc("/api/v5/audiobooks/badstatus/playlists.json", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	scraper.BaseURL = server.URL
	_, err = scraper.fetchPlaylists(context.Background(), "badjson", "token")
	if err == nil {
		t.Fatal("fetchPlaylists() expected json decode error")
	}
	_, err = scraper.fetchPlaylists(context.Background(), "badstatus", "token")
	if err == nil {
		t.Fatal("fetchPlaylists() expected status error")
	}
}

func TestGetToken_Errors(t *testing.T) {
	t.Setenv("BOOKS_YANDEX_TOKEN", "")

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("APPDATA", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	configPath := filepath.Join(tmpDir, "audiobook-tools", "books_yandex.json")
	if errMkdir := os.MkdirAll(filepath.Dir(configPath), 0o700); errMkdir != nil {
		t.Fatalf("MkdirAll error: %v", errMkdir)
	}

	if errWrite := os.WriteFile(configPath, []byte(`{bad}`), 0o600); errWrite != nil {
		t.Fatalf("WriteFile error: %v", errWrite)
	}
	_, err := GetToken()
	if err == nil {
		t.Fatal("GetToken() expected error for bad JSON")
	}

	if errWrite := os.WriteFile(configPath, []byte(`{"token":""}`), 0o600); errWrite != nil {
		t.Fatalf("WriteFile error: %v", errWrite)
	}
	_, err = GetToken()
	if err == nil {
		t.Fatal("GetToken() expected error for empty token")
	}

	if errRemove := os.Remove(configPath); errRemove != nil && !os.IsNotExist(errRemove) {
		t.Fatalf("Remove error: %v", errRemove)
	}
	_, err = GetToken()
	if err == nil {
		t.Fatal("GetToken() expected error for missing file")
	}

	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("APPDATA", "")
	t.Setenv("USERPROFILE", "")
	_, err = GetToken()
	if err == nil {
		t.Fatal("GetToken() expected error for missing config dir")
	}

	_, err = SaveToken("token")
	if err == nil {
		t.Fatal("SaveToken() expected error for missing config dir")
	}
}

func TestBooksYandex_NewRequestErrors(t *testing.T) {
	scraper := NewBooksYandex()
	scraper.BaseURL = string([]byte{0x7f})

	_, err := scraper.fetchInfo(context.Background(), "uuid", "token")
	if err == nil {
		t.Fatal("fetchInfo() expected error on NewRequest")
	}

	_, err = scraper.fetchPlaylists(context.Background(), "uuid", "token")
	if err == nil {
		t.Fatal("fetchPlaylists() expected error on NewRequest")
	}
}
