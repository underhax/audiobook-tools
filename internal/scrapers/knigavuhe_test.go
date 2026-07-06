package scrapers

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestKnigavuheGetBookInfo(t *testing.T) {
	htmlContent := `
	<html>
	<body>
		<div class="page_title">
			<span itemprop="name">The Great Book</span>
			<span itemprop="author">John Doe</span>
			<span class="book_title_elem">читает Jane Doe</span>
		</div>
		<div class="book_description">A very long description.</div>
		<div class="book_cover"><img src="http://example.com/cover.jpg"></div>
		<div class="book_genre_pretitle">Fantasy, Adventure</div>
		<script>
			var player = new BookPlayer([{"url":"http://example.com/1.mp3","title":"Chapter 1"}, {"url":"http://example.com/2.mp3","title":"Chapter 2"}]);
		</script>
	</body>
	</html>
	`

	scraper := NewKnigavuhe()
	info, chapters, err := scraper.GetBookInfo(context.Background(), htmlContent, "https://example.com/book/test-book/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Title != "The Great Book" {
		t.Errorf("expected title 'The Great Book', got '%s'", info.Title)
	}
	if info.Author != "John Doe" {
		t.Errorf("expected author 'John Doe', got '%s'", info.Author)
	}
	if info.Narrator != "Jane Doe" {
		t.Errorf("expected narrator 'Jane Doe', got '%s'", info.Narrator)
	}
	if info.Description != "A very long description." {
		t.Errorf("expected description 'A very long description.', got '%s'", info.Description)
	}
	if info.Cover != "http://example.com/cover.jpg" {
		t.Errorf("expected cover URL 'http://example.com/cover.jpg', got '%s'", info.Cover)
	}
	if len(info.Genres) != 2 || info.Genres[0] != "fantasy" || info.Genres[1] != "adventure" {
		t.Errorf("expected genres [fantasy, adventure], got %v", info.Genres)
	}

	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chapters))
	}
	if chapters[0].Title != "Chapter 1" || chapters[0].URL != "http://example.com/1.mp3" {
		t.Errorf("unexpected chapter 1: %+v", chapters[0])
	}
}

func TestKnigavuheGetBookInfo_MissingParts(t *testing.T) {
	htmlContent := `
	<html>
	<body>
		<div class="page_title">
			<span itemprop="name">The Great Book</span>
		</div>
	</body>
	</html>
	`

	scraper := NewKnigavuhe()
	_, _, err := scraper.GetBookInfo(context.Background(), htmlContent, "http://example.com/book")
	if err == nil {
		t.Fatal("expected error for missing BookPlayer, got nil")
	}
}

func TestExtractJSONPlaylist_Error(t *testing.T) {
	_, err := extractJSONPlaylist("No book player here")
	if err == nil {
		t.Error("expected error for missing BookPlayer, got nil")
	}

	_, err = extractJSONPlaylist("BookPlayer is here but no array")
	if err == nil {
		t.Error("expected error for missing JSON array, got nil")
	}

	_, err = extractJSONPlaylist("BookPlayer is here [{ no end")
	if err == nil {
		t.Error("expected error for missing JSON array end, got nil")
	}

	_, err = extractJSONPlaylist("BookPlayer is here [{invalid json}]")
	if err == nil {
		t.Error("expected error for invalid json, got nil")
	}
}

func TestKnigavuheGetBookInfo_ParseError(t *testing.T) {
	orig := stringsNewReader
	defer func() { stringsNewReader = orig }()

	stringsNewReader = func(_ string) io.Reader {
		return errReader(0)
	}

	scraper := NewKnigavuhe()
	_, _, err := scraper.GetBookInfo(context.Background(), "html", "https://example.com/book")
	if err == nil || !strings.Contains(err.Error(), "mock read error") {
		t.Fatalf("expected error containing 'mock read error', got %v", err)
	}
}
