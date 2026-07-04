// Package scrapers provides HTML parsers for various audiobook websites.
package scrapers

import (
	"context"

	"github.com/underhax/audiobook-tools/internal/core"
)

// Scraper defines the interface for an audiobook website scraper.
type Scraper interface {
	GetBookInfo(ctx context.Context, htmlContent string, bookURL string) (core.BookInfo, []core.Chapter, error)
}
