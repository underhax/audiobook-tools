package scrapers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/underhax/audiobook-tools/internal/core"
	"golang.org/x/net/html"
)

const (
	tagSpan = "span"
	tagDiv  = "div"
)

// Knigavuhe isolates the HTML parsing logic specifically for the knigavuhe.org DOM structure.
type Knigavuhe struct{}

// NewKnigavuhe instantiates the knigavuhe.org parser to fulfill the Scraper interface requirements.
func NewKnigavuhe() *Knigavuhe {
	return &Knigavuhe{}
}

// SetClient implements the Scraper interface. Knigavuhe does not make HTTP requests.
func (s *Knigavuhe) SetClient(client *http.Client) {
	_ = client
}

// GetBookInfo translates the raw HTML response into structured domain models for downstream processing.
func (s *Knigavuhe) GetBookInfo(_ context.Context, htmlContent, bookURL string) (core.BookInfo, []core.Chapter, error) {
	doc, err := html.Parse(stringsNewReader(htmlContent))
	if err != nil {
		return core.BookInfo{}, nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	info := core.BookInfo{
		URL: bookURL,
	}

	extractNodes(doc, &info)

	if info.Author == "" {
		info.Author = authorUnknown
		info.Authors = append(info.Authors, info.Author)
	}

	files, err := extractJSONPlaylist(htmlContent)
	if err != nil {
		return core.BookInfo{}, nil, err
	}

	return info, files, nil
}

func extractNodes(n *html.Node, info *core.BookInfo) {
	if n.Type == html.ElementNode {
		processElementNode(n, info)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractNodes(c, info)
	}
}

func processElementNode(n *html.Node, info *core.BookInfo) {
	switch n.Data {
	case tagSpan:
		processSpan(n, info)
	case tagDiv:
		processDiv(n, info)
	}
}

func processSpan(n *html.Node, info *core.BookInfo) {
	switch getAttribute(n, "itemprop") {
	case "name":
		if title := getText(n); title != "" && info.Title == "" {
			info.Title = title
		}
	case "author":
		if author := getText(n); author != "" && info.Author == "" {
			info.Author = author
			info.Authors = append(info.Authors, author)
		}
	}

	if getAttribute(n, "class") == "book_title_elem" {
		text := getText(n)
		if strings.Contains(text, "читает") {
			narrator := strings.TrimSpace(strings.ReplaceAll(text, "читает", ""))
			if narrator != "" && info.Narrator == "" {
				info.Narrator = narrator
				info.Narrators = append(info.Narrators, narrator)
			}
		}
	}
}

func processDiv(n *html.Node, info *core.BookInfo) {
	switch getAttribute(n, "class") {
	case "book_description":
		if desc := getText(n); desc != "" && info.Description == "" {
			info.Description = desc
		}
	case "book_cover":
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "img" {
				if src := getAttribute(c, "src"); src != "" {
					info.Cover = src
					break
				}
			}
		}
	case "book_genre_pretitle":
		if genreText := getText(n); genreText != "" && len(info.Genres) == 0 {
			for p := range strings.SplitSeq(genreText, ",") {
				info.Genres = append(info.Genres, strings.ToLower(strings.TrimSpace(p)))
			}
		}
	}
}

func getAttribute(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func getText(n *html.Node) string {
	var buf strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return strings.Join(strings.Fields(buf.String()), " ")
}

func extractJSONPlaylist(htmlContent string) ([]core.Chapter, error) {
	parts := strings.Split(htmlContent, "BookPlayer")
	if len(parts) < 2 {
		return nil, errors.New("could not find BookPlayer in HTML")
	}

	jsonParts := strings.Split(parts[len(parts)-1], "[{")
	if len(jsonParts) < 2 {
		return nil, errors.New("could not find JSON array start")
	}

	jsonStr := "[{" + jsonParts[1]
	endIdx := strings.Index(jsonStr, "}]")
	if endIdx == -1 {
		return nil, errors.New("could not find JSON array end")
	}
	jsonStr = jsonStr[:endIdx+2]

	var chapters []core.Chapter
	if err := json.Unmarshal([]byte(jsonStr), &chapters); err != nil {
		return nil, fmt.Errorf("failed to parse playlist JSON: %w", err)
	}

	return chapters, nil
}
