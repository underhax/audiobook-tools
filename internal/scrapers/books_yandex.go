package scrapers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/underhax/audiobook-tools/internal/core"
	"github.com/underhax/audiobook-tools/pkg/utils/httputil"
)

// BooksYandex represents the BooksYandex website scraper/API client.
type BooksYandex struct {
	Client  *http.Client
	BaseURL string
}

// NewBooksYandex initializes a new BooksYandex scraper.
func NewBooksYandex() *BooksYandex {
	return &BooksYandex{
		Client: &http.Client{
			Transport: &httputil.RetryTransport{MaxRetries: 3},
		},
		BaseURL: "https://api.bookmate.yandex.net",
	}
}

// GetToken returns the BooksYandex token from the environment variable or config file.
func GetToken() (string, error) {
	token := os.Getenv("BOOKS_YANDEX_TOKEN")
	if token != "" {
		return token, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}

	configPath := filepath.Join(configDir, "audiobook-tools", "books_yandex.json")
	data, err := os.ReadFile(filepath.Clean(configPath))
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("token not found")
		}
		return "", fmt.Errorf("read config file: %w", err)
	}

	var config struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("parse config file: %w", err)
	}

	if config.Token == "" {
		return "", fmt.Errorf("token is empty in %s", configPath)
	}

	return config.Token, nil
}

// SaveToken saves the BooksYandex token to the user config directory and returns the path.
func SaveToken(token string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}

	appDir := filepath.Join(configDir, "audiobook-tools")
	const dirPerm = 0o700
	if errMkdir := os.MkdirAll(appDir, dirPerm); errMkdir != nil {
		return "", fmt.Errorf("create config dir: %w", errMkdir)
	}

	configPath := filepath.Join(appDir, "books_yandex.json")
	data, err := json.MarshalIndent(map[string]string{"token": token}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}

	const filePerm = 0o600
	if errWrite := os.WriteFile(configPath, data, filePerm); errWrite != nil {
		return "", fmt.Errorf("write config file: %w", errWrite)
	}

	return configPath, nil
}

// extractUUID extracts the UUID from a BooksYandex URL.
func extractUUID(bookURL string) (string, error) {
	re := regexp.MustCompile(`audiobooks/([^/?]+)`)
	matches := re.FindStringSubmatch(bookURL)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract UUID from URL: %s", bookURL)
	}
	return matches[1], nil
}

// BooksYandexInfo represents the JSON response for audiobook info.
type BooksYandexInfo struct {
	Audiobook struct {
		Title string `json:"title"`
		Cover struct {
			Large string `json:"large"`
		} `json:"cover"`
		Annotation string `json:"annotation"`
		Authors    []struct {
			Name string `json:"name"`
		} `json:"authors"`
		Narrators []struct {
			Name string `json:"name"`
		} `json:"narrators"`
		SeriesList []struct {
			Title string `json:"title"`
		} `json:"series_list"`
		Topics []struct {
			Title string `json:"title"`
		} `json:"topics"`
		PublicationDate int64 `json:"publication_date"`
	} `json:"audiobook"`
}

// BooksYandexPlaylists represents the JSON response for the playlists.
type BooksYandexPlaylists struct {
	Tracks []struct {
		Offline struct {
			MaxBitRate struct {
				URL string `json:"url"`
			} `json:"max_bit_rate"`
		} `json:"offline"`
		Number int `json:"number"`
	} `json:"tracks"`
}

// GetBookInfo fetches audiobook metadata and track playlists from BooksYandex API.
func (b *BooksYandex) GetBookInfo(ctx context.Context, _, bookURL string) (core.BookInfo, []core.Chapter, error) {
	uuid, err := extractUUID(bookURL)
	if err != nil {
		return core.BookInfo{}, nil, err
	}

	token, err := GetToken()
	if err != nil {
		return core.BookInfo{}, nil, &core.AuthError{
			ProviderName: "Яндекс Книги",
			ProviderID:   "books_yandex",
			EnvVar:       "BOOKS_YANDEX_TOKEN",
		}
	}

	info, err := b.fetchInfo(ctx, uuid, token)
	if err != nil {
		return core.BookInfo{}, nil, fmt.Errorf("fetch info: %w", err)
	}

	chapters, err := b.fetchPlaylists(ctx, uuid, token)
	if err != nil {
		return info, nil, fmt.Errorf("fetch playlists: %w (please check your token or subscription)", err)
	}

	return info, chapters, nil
}

func handleBooksYandexResponse(method, url string, req *http.Request, resp *http.Response) ([]byte, error) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if os.Getenv("DEBUG") != "" {
		debugHeaders := req.Header.Clone()
		if debugHeaders.Get("Auth-Token") != "" {
			debugHeaders.Set("Auth-Token", "[HIDDEN]")
		}
		log.Printf("[DEBUG] %s URL: %q", method, url)
		log.Printf("[DEBUG] %s Status: %d", method, resp.StatusCode)
		safeHeaders := strings.ReplaceAll(fmt.Sprintf("%v", debugHeaders), "\n", "")
		safeHeaders = strings.ReplaceAll(safeHeaders, "\r", "")
		log.Printf("[DEBUG] %s Request Headers: %q", method, safeHeaders)
		log.Printf("[DEBUG] %s Response: %q", method, string(bodyBytes))
	}
	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if parseErr := json.Unmarshal(bodyBytes, &apiErr); parseErr == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("API error: %s", apiErr.Error.Message)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}
	return bodyBytes, nil
}

func (b *BooksYandex) fetchInfo(ctx context.Context, uuid, token string) (core.BookInfo, error) {
	url := fmt.Sprintf("%s/api/v5/audiobooks/%s", b.BaseURL, uuid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return core.BookInfo{}, fmt.Errorf("create info request: %w", err)
	}

	setBooksYandexHeaders(req, token)

	resp, err := b.Client.Do(req)
	if err != nil {
		return core.BookInfo{}, fmt.Errorf("do info request: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("failed to close response body: %v", cerr)
		}
	}()

	bodyBytes, err := handleBooksYandexResponse("fetchInfo", url, req, resp)
	if err != nil {
		return core.BookInfo{}, err
	}

	var data BooksYandexInfo
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return core.BookInfo{}, fmt.Errorf("decode json: %w", err)
	}

	author := "Unknown"
	if len(data.Audiobook.Authors) > 0 {
		author = data.Audiobook.Authors[0].Name
	}

	narrator := ""
	if len(data.Audiobook.Narrators) > 0 {
		narrator = data.Audiobook.Narrators[0].Name
	}

	yearStr := ""
	if data.Audiobook.PublicationDate > 0 {
		yearStr = strconv.Itoa(time.Unix(data.Audiobook.PublicationDate, 0).Year())
	}

	var authorsList []string
	for _, a := range data.Audiobook.Authors {
		authorsList = append(authorsList, a.Name)
	}

	var narratorsList []string
	for _, n := range data.Audiobook.Narrators {
		narratorsList = append(narratorsList, n.Name)
	}

	var genresList []string
	for _, t := range data.Audiobook.Topics {
		if t.Title != "" {
			genresList = append(genresList, t.Title)
		}
	}

	return core.BookInfo{
		URL:           "https://books.yandex.ru/audiobooks/" + uuid,
		Title:         data.Audiobook.Title,
		Author:        author,
		Authors:       authorsList,
		Cover:         data.Audiobook.Cover.Large,
		Description:   data.Audiobook.Annotation,
		Narrator:      narrator,
		Narrators:     narratorsList,
		Genres:        genresList,
		PublishedYear: yearStr,
	}, nil
}

func (b *BooksYandex) fetchPlaylists(ctx context.Context, uuid, token string) ([]core.Chapter, error) {
	url := fmt.Sprintf("%s/api/v5/audiobooks/%s/playlists.json", b.BaseURL, uuid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create playlists request: %w", err)
	}

	setBooksYandexHeaders(req, token)

	resp, err := b.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do playlists request: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("failed to close response body: %v", cerr)
		}
	}()

	bodyBytes, err := handleBooksYandexResponse("fetchPlaylists", url, req, resp)
	if err != nil {
		return nil, err
	}

	var playlistsData BooksYandexPlaylists
	if err := json.Unmarshal(bodyBytes, &playlistsData); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	var chapters []core.Chapter
	for _, track := range playlistsData.Tracks {
		trackURL := track.Offline.MaxBitRate.URL
		if trackURL == "" {
			continue
		}

		trackURL = strings.Replace(trackURL, ".m3u8", ".m4a", 1)

		chapters = append(chapters, core.Chapter{
			URL:       trackURL,
			Title:     fmt.Sprintf("Chapter %d", track.Number+1),
			Extension: ".m4a",
		})
	}

	return chapters, nil
}

var booksYandexUAs = []string{
	"Samsung/Galaxy_S24 Android/14 Bookmate/4.12.0",
	"Google/Pixel_8 Android/14 Bookmate/4.12.0",
	"Xiaomi/14_Pro Android/14 Bookmate/4.12.0",
	"OnePlus/12 Android/14 Bookmate/4.12.0",
	"Samsung/Galaxy_A55 Android/14 Bookmate/4.12.0",
	"Huawei/P60_Pro Android/13 Bookmate/4.12.0",
}

func setBooksYandexHeaders(req *http.Request, token string) {
	req.Header.Set("auth-token", token)

	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(booksYandexUAs))))
	userAgent := booksYandexUAs[0]
	if err == nil {
		userAgent = booksYandexUAs[n.Int64()]
	}

	req.Header.Set("app-user-agent", userAgent)
	req.Header.Set("mcc", "")
	req.Header.Set("mnc", "")
	req.Header.Set("imei", "")
	req.Header.Set("subscription-country", "")
	req.Header.Set("app-locale", "")
	req.Header.Set("bookmate-version", "")
	req.Header.Set("bookmate-websocket-version", "")
	req.Header.Set("device-idfa", "")
	req.Header.Set("onyx-preinstall", "false")
	req.Header.Set("accept-encoding", "")
	req.Header.Set("user-agent", "")
}
