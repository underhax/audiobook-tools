package scrapers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	userConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { userConfigDir = defaultUserConfigDir }()

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
	userConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { userConfigDir = defaultUserConfigDir }()

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
	userConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { userConfigDir = defaultUserConfigDir }()

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
	scraper.SetClient(&http.Client{Transport: http.DefaultTransport})
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
	scraper.SetClient(&http.Client{Transport: http.DefaultTransport})
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
	userConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { userConfigDir = defaultUserConfigDir }()

	configPath := filepath.Join(tmpDir, "audiobook-tools", "books_yandex.json")
	if errMkdir := os.MkdirAll(filepath.Dir(configPath), 0o700); errMkdir != nil {
		t.Fatalf("MkdirAll error: %v", errMkdir)
	}

	if errMkdir := os.MkdirAll(configPath, 0o700); errMkdir != nil {
		t.Fatalf("MkdirAll configPath error: %v", errMkdir)
	}
	_, err := GetToken()
	if err == nil {
		t.Fatal("GetToken() expected error for read config file error")
	}
	if errRemove := os.Remove(configPath); errRemove != nil {
		t.Fatalf("Remove configPath error: %v", errRemove)
	}

	if errWrite := os.WriteFile(configPath, []byte(`{bad}`), 0o600); errWrite != nil {
		t.Fatalf("WriteFile error: %v", errWrite)
	}
	_, err = GetToken()
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

	userConfigDir = func() (string, error) { return "", errors.New("no config dir") }
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

func TestSaveToken_Errors(t *testing.T) {
	tmpDir := t.TempDir()

	userConfigDir = func() (string, error) { return filepath.Join(tmpDir, "file.txt"), nil }
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("data"), 0o600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	_, err := SaveToken("token")
	if err == nil {
		t.Fatal("expected MkdirAll error")
	}

	userConfigDir = func() (string, error) { return tmpDir, nil }
	jsonMarshalIndent = func(_ any, _, _ string) ([]byte, error) {
		return nil, errors.New("mock json error")
	}
	_, err = SaveToken("token")
	if err == nil {
		t.Fatal("expected json marshal error")
	}
	jsonMarshalIndent = defaultJSONMarshalIndent

	configPath := filepath.Join(tmpDir, "audiobook-tools", "books_yandex.json")
	if errMkdir := os.MkdirAll(configPath, 0o700); errMkdir != nil {
		t.Fatalf("MkdirAll configPath error: %v", errMkdir)
	}
	_, err = SaveToken("token")
	if err == nil {
		t.Fatal("expected WriteFile error")
	}
}

func TestBooksYandex_GetBookInfo_FetchInfoError(t *testing.T) {
	t.Setenv("BOOKS_YANDEX_TOKEN", "dummy-token")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v5/audiobooks/testuuid", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	scraper := NewBooksYandex()

	scraper.BaseURL = server.URL

	_, _, err := scraper.GetBookInfo(context.Background(), "", "https://example.com/audiobooks/testuuid")
	if err == nil {
		t.Fatal("GetBookInfo() expected error on fetchInfo failure")
	}
}

func TestBooksYandex_GetBookInfo_FetchPlaylistsError(t *testing.T) {
	t.Setenv("BOOKS_YANDEX_TOKEN", "dummy-token")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v5/audiobooks/testuuid", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"audiobook": {"can_be_listened": true}}`)); err != nil {
			t.Errorf("w.Write error: %v", err)
		}
	})
	mux.HandleFunc("/api/v5/audiobooks/testuuid/playlists.json", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	scraper := NewBooksYandex()

	scraper.BaseURL = server.URL

	_, _, err := scraper.GetBookInfo(context.Background(), "", "https://example.com/audiobooks/testuuid")
	if err == nil {
		t.Fatal("GetBookInfo() expected error on fetchPlaylists failure")
	}
}

type errReader int

func (errReader) Read(_ []byte) (n int, err error) {
	return 0, errors.New("mock read error")
}
func (errReader) Close() error { return nil }

func TestHandleBooksYandexResponse_ReadError(t *testing.T) {
	resp := &http.Response{
		Body: errReader(0),
	}
	_, err := handleBooksYandexResponse("GET", "http://example.com", nil, resp)
	if err == nil {
		t.Fatal("expected read error")
	}
}

func TestHandleBooksYandexResponse_DebugLog(t *testing.T) {
	t.Setenv("DEBUG", "1")
	req, errReq := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", http.NoBody)
	if errReq != nil {
		t.Fatalf("unexpected request error: %v", errReq)
	}
	req.Header["Test-Header"] = []string{"test\nvalue\r"}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("ok")),
	}

	_, err := handleBooksYandexResponse("GET", "http://example.com", req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleBooksYandexResponse_JSONError(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(`{"error": {"message": "Invalid token"}}`)),
	}
	_, err := handleBooksYandexResponse("GET", "http://example.com", nil, resp)
	if err == nil {
		t.Fatal("expected API error")
	}
	if !strings.Contains(err.Error(), "Invalid token") {
		t.Fatalf("expected error to contain 'Invalid token', got: %v", err)
	}
}

func TestBooksYandex_fetchInfo_NotListenable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v5/audiobooks/testuuid", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"audiobook": {"can_be_listened": false}}`)); err != nil {
			t.Errorf("w.Write error: %v", err)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	scraper := NewBooksYandex()

	scraper.BaseURL = server.URL

	_, err := scraper.fetchInfo(context.Background(), "testuuid", "token")
	if err == nil {
		t.Fatal("expected error for can_be_listened = false")
	}
	if !strings.Contains(err.Error(), "current subscription does not allow") {
		t.Fatalf("expected subscription error, got: %v", err)
	}
}

type errCloseReader int

func (errCloseReader) Read(_ []byte) (n int, err error) {
	return 0, io.EOF
}
func (errCloseReader) Close() error { return errors.New("mock close error") }

type mockRoundTripper struct {
	resp *http.Response
	err  error
}

func (m *mockRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return m.resp, m.err
}

func TestBooksYandex_fetchInfo_CloseError(t *testing.T) {
	scraper := NewBooksYandex()
	scraper.SetClient(&http.Client{Transport: &mockRoundTripper{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       errCloseReader(0),
		},
	}})
	scraper.BaseURL = "http://example.net"

	_, err := scraper.fetchInfo(context.Background(), "testuuid", "token")
	if err == nil {
		t.Fatal("expected error from fetchInfo due to EOF")
	}
}

func TestMapBooksYandexInfoToBookInfo_Full(t *testing.T) {
	jsonStr := `{
		"audiobook": {
			"title": "Test Title",
			"annotation": "Test Annotation",
			"language": "ru",
			"age_restriction": "18+",
			"can_be_listened": true,
			"publication_date": 1609459200,
			"cover": {
				"large": "cover.jpg"
			},
			"authors": [{"name": "Author 1"}, {"name": "Author 2"}],
			"narrators": [{"name": "Narrator 1"}, {"name": "Narrator 2"}],
			"topics": [{"title": "Topic 1"}, {"title": ""}],
			"publishers": [{"name": "Publisher 1"}],
			"series_list": [{"title": "Series 1"}],
			"translators": [{"name": "Translator 1"}]
		}
	}`
	var data BooksYandexInfo
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	info := mapBooksYandexInfoToBookInfo("uuid123", &data)
	if info.Title != "Test Title" {
		t.Errorf("expected Title 'Test Title', got %q", info.Title)
	}
	if info.Author != "Author 1" || len(info.Authors) != 2 {
		t.Errorf("unexpected authors")
	}
	if info.Narrator != "Narrator 1" || len(info.Narrators) != 2 {
		t.Errorf("unexpected narrators")
	}
	if len(info.Genres) != 1 || info.Genres[0] != "Topic 1" {
		t.Errorf("unexpected genres")
	}
	if info.Publisher != "Publisher 1" {
		t.Errorf("unexpected publisher")
	}
	if info.Series != "Series 1" {
		t.Errorf("unexpected series")
	}
	if len(info.Translators) != 1 || info.Translators[0] != "Translator 1" {
		t.Errorf("unexpected translators")
	}
	if info.PublishedYear != "2021" {
		t.Errorf("expected year 2021, got %q", info.PublishedYear)
	}
}

func TestBooksYandex_fetchPlaylists_CloseError(t *testing.T) {
	scraper := NewBooksYandex()
	scraper.SetClient(&http.Client{Transport: &mockRoundTripper{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       errCloseReader(0),
		},
	}})
	scraper.BaseURL = "http://example.com"

	_, err := scraper.fetchPlaylists(context.Background(), "testuuid", "token")
	if err == nil {
		t.Fatal("expected error from fetchPlaylists due to EOF")
	}
}

func TestBooksYandex_fetchPlaylists_EmptyTrackURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v5/audiobooks/testuuid/playlists.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{
			"tracks": [
				{
					"offline": {
						"max_bit_rate": {
							"url": ""
						}
					}
				},
				{
					"offline": {
						"max_bit_rate": {
							"url": "http://example.com/track.m3u8"
						}
					}
				}
			]
		}`)); err != nil {
			t.Errorf("w.Write error: %v", err)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	scraper := NewBooksYandex()

	scraper.BaseURL = server.URL

	chapters, err := scraper.fetchPlaylists(context.Background(), "testuuid", "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chapters) != 1 {
		t.Fatalf("expected 1 chapter, got %d", len(chapters))
	}
	if chapters[0].URL != "http://example.com/track.m4a" {
		t.Errorf("unexpected URL: %q", chapters[0].URL)
	}
}
