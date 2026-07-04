package scrapers

import (
	"context"
	"strings"
	"testing"
)

func TestDetiOnlineGetBookInfo(t *testing.T) {
	htmlContent := `
	<html>
	<body>
		<h1>Аудиосказка «Test Book»</h1>
		<div class="hint">Автор: Test Author</div>
		<div class="lead cf"><p>Аудиосказка &laquo;Test Book&raquo; &mdash; сборник рассказов...</p></div>
		<picture id="main-img" title="Test Book"><source type="image/webp" srcset="/test-cover.webp 1x, /@2x/test-cover.webp 2x"><img src="/test-cover.jpg" srcset="/@2x/test-cover.jpg 2x" width="250" height="250" alt="Аудиосказка Test Book" loading="lazy"></picture>
		<ul class="playlist grid grid-list">
			<li class="placeholder item item-231 is-ripple" data-f="grfg" data-id="231" data-g="187">
			  <div class="counter">1</div>
			  <div class="name"><span class="t">Test Chapter</span></div>
			  <div class="time">29:07</div>
			  <button class="btn-icon download" title="Скачать"></button>
			  <i class="dot"></i>
			</li>
		</ul>
	</body>
	</html>
	`

	s := NewDetiOnline()
	info, chapters, err := s.GetBookInfo(context.Background(), htmlContent, "https://example.com/audioknigi/test-book/")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Title != "Test Book" {
		t.Errorf("expected Title 'Test Book', got '%s'", info.Title)
	}
	if info.Author != "Test Author" {
		t.Errorf("expected Author 'Test Author', got '%s'", info.Author)
	}
	if !strings.Contains(info.Description, "Аудиосказка") {
		t.Errorf("expected Description to contain 'Аудиосказка', got '%s'", info.Description)
	}
	if !strings.HasSuffix(info.Cover, "/test-cover.jpg") {
		t.Errorf("expected Cover to end with '/test-cover.jpg', got '%s'", info.Cover)
	}

	if len(chapters) != 1 {
		t.Fatalf("expected 1 chapter, got %d", len(chapters))
	}

	if chapters[0].Title != "Test Chapter" {
		t.Errorf("expected Chapter 0 Title 'Test Chapter', got '%s'", chapters[0].Title)
	}

	if !strings.HasSuffix(chapters[0].URL, "/s/test.mp3") {
		t.Errorf("expected Chapter 0 URL to end with '/s/test.mp3', got '%s'", chapters[0].URL)
	}
}

func TestRot13(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"nhqvb/obbx/o/po", "audio/book/b/cb"},
		{"FUsaQIig60NiZW9hWALxeD", "SHfnDVvt60AvMJ9uJNYkrQ"},
	}

	for _, c := range cases {
		got := rot13(c.in)
		if got != c.want {
			t.Errorf("rot13(%q) == %q, want %q", c.in, got, c.want)
		}
	}
}
