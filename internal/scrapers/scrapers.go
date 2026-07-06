// Package scrapers provides HTML parsers for various audiobook websites.
package scrapers

import (
	"context"

	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/underhax/audiobook-tools/internal/core"
)

// Scraper defines the behavior required for any audiobook source parser.
type Scraper interface {
	GetBookInfo(ctx context.Context, htmlContent, bookURL string) (core.BookInfo, []core.Chapter, error)
	SetClient(client *http.Client)
}

const authorUnknown = "Автор неизвестен"

func defaultStringsNewReader(s string) io.Reader {
	return strings.NewReader(s)
}

var (
	stringsNewReader  = defaultStringsNewReader
	userConfigDir     = defaultUserConfigDir
	jsonMarshalIndent = defaultJSONMarshalIndent
)

func defaultUserConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("os config dir: %w", err)
	}
	return dir, nil
}

func defaultJSONMarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(v, prefix, indent)
}
