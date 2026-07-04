package scrapers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/underhax/audiobook-tools/internal/core"
	"golang.org/x/net/html"
)

// DetiOnline isolates the HTML parsing logic for deti-online.com
type DetiOnline struct {
	Version int
}

// NewDetiOnline instantiates the deti-online.com parser.
func NewDetiOnline() *DetiOnline {
	return &DetiOnline{Version: 1}
}

// GetBookInfo translates the raw HTML response into structured domain models.
func (s *DetiOnline) GetBookInfo(ctx context.Context, htmlContent, bookURL string) (core.BookInfo, []core.Chapter, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return core.BookInfo{}, nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	info := core.BookInfo{
		URL: bookURL,
	}

	var chapters []core.Chapter

	var playerJSURL string
	extractDetiNodes(doc, &info, &chapters, &playerJSURL, s.Version)

	if info.Author == "" {
		info.Author = "Автор неизвестен"
		info.Authors = append(info.Authors, info.Author)
	}

	if info.Title != "" {
		prefixes := []string{"Аудиосказка «", "Аудиокнига «", "Аудиосказка ", "Аудиокнига "}
		for _, prefix := range prefixes {
			if trimmed, found := strings.CutPrefix(info.Title, prefix); found {
				info.Title = trimmed
				break
			}
		}
		info.Title = strings.TrimSuffix(info.Title, "»")
		info.Title = strings.TrimSpace(info.Title)
	}

	serverDomain := getServerDomain(ctx, playerJSURL)

	for i := range chapters {
		chapters[i].URL = strings.ReplaceAll(chapters[i].URL, "{SERVER}", serverDomain)
	}

	return info, chapters, nil
}

func getServerDomain(ctx context.Context, playerJSURL string) string {
	fallback := "stat4.deti-online.com"
	if playerJSURL == "" {
		return fallback
	}

	if !strings.HasPrefix(playerJSURL, "http") {
		playerJSURL = "https://deti-online.com" + playerJSURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, playerJSURL, http.NoBody)
	if err != nil {
		return fallback
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fallback
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			fmt.Printf("failed to close body: %v\n", cerr)
		}
	}()

	if jsBytes, err := io.ReadAll(resp.Body); err == nil {
		re := regexp.MustCompile(`(stat\d+\.deti-online\.com)`)
		if match := re.FindString(string(jsBytes)); match != "" {
			return match
		}
	}
	return fallback
}

func extractDetiNodes(n *html.Node, info *core.BookInfo, chapters *[]core.Chapter, playerJSURL *string, version int) {
	if n.Type == html.ElementNode {
		if n.Data == "li" {
			class := getAttribute(n, "class")
			if strings.Contains(class, "group") && !isCorrectVersion(class, version) {
				return
			}
		}
		processDetiElement(n, info, chapters, playerJSURL)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractDetiNodes(c, info, chapters, playerJSURL, version)
	}
}

func isCorrectVersion(class string, version int) bool {
	targetClass := fmt.Sprintf("t-%d", version)
	hasTClass := false
	isTarget := false
	for c := range strings.FieldsSeq(class) {
		if strings.HasPrefix(c, "t-") && len(c) > 2 {
			hasTClass = true
		}
		if c == targetClass {
			isTarget = true
		}
	}

	if hasTClass {
		return isTarget
	}
	return version <= 1
}

func processDetiElement(n *html.Node, info *core.BookInfo, chapters *[]core.Chapter, playerJSURL *string) {
	switch n.Data {
	case "script":
		src := getAttribute(n, "src")
		if strings.Contains(src, "audio-player") {
			*playerJSURL = src
		}
	case "h1":
		if info.Title == "" {
			info.Title = getText(n)
		}
	case "div":
		processDetiDiv(n, info)
	case "picture":
		if getAttribute(n, "id") == "main-img" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "img" {
					if src := getAttribute(c, "src"); src != "" {
						info.Cover = "https://deti-online.com" + src
					}
				}
			}
		}
	case "li":
		processDetiLi(n, chapters)
	}
}

func processDetiDiv(n *html.Node, info *core.BookInfo) {
	class := getAttribute(n, "class")
	if class == "hint" && strings.HasPrefix(getText(n), "Автор:") {
		author := strings.TrimSpace(strings.TrimPrefix(getText(n), "Автор:"))
		if info.Author == "" && author != "" {
			info.Author = author
			info.Authors = append(info.Authors, author)
		}
	} else if class == "lead cf" {
		desc := getText(n)
		if info.Description == "" && desc != "" {
			info.Description = desc
		}
	}
}

func processDetiLi(n *html.Node, chapters *[]core.Chapter) {
	class := getAttribute(n, "class")
	if !strings.Contains(class, "item") || !strings.Contains(class, "is-ripple") {
		return
	}

	dataF := getAttribute(n, "data-f")
	if dataF == "" {
		return
	}

	url := decodeDetiURL(dataF)
	var title string

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "div" && getAttribute(c, "class") == "name" {
			for gc := c.FirstChild; gc != nil; gc = gc.NextSibling {
				if gc.Type == html.ElementNode && gc.Data == "span" && getAttribute(gc, "class") == "t" {
					title = getText(gc)
				}
			}
		}
	}

	if title == "" {
		title = fmt.Sprintf("Chapter %d", len(*chapters)+1)
	}

	*chapters = append(*chapters, core.Chapter{
		Title: title,
		URL:   url,
	})
}

func decodeDetiURL(dataF string) string {
	decoded := rot13(dataF)
	return "https://{SERVER}/s/" + decoded + ".mp3"
}

func rot13(s string) string {
	var result []rune
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			if r+13 > 'z' {
				result = append(result, r-13)
			} else {
				result = append(result, r+13)
			}
		case r >= 'A' && r <= 'Z':
			if r+13 > 'Z' {
				result = append(result, r-13)
			} else {
				result = append(result, r+13)
			}
		default:
			result = append(result, r)
		}
	}
	return string(result)
}
