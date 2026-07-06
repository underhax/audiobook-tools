package scrapers

import (
	"context"
	"errors"
	"io"
	"net/http"
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

type detiMockRoundTripper struct {
	resp *http.Response
	err  error
}

func (m *detiMockRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return m.resp, m.err
}

func TestDetiOnlineGetBookInfo_Coverage(t *testing.T) {
	htmlContent := `
	<html>
	<body>
		<li class="group t-2">Group 2</li>
		<li class="group t-1">Group 1</li>
		<li class="group">Group 3</li>
		<li class="item is-ripple"></li>
		<li class="item is-ripple" data-f="grfg"></li>
		<script src="/audio-player.js"></script>
	</body>
	</html>
	`
	s := NewDetiOnline()
	s.Client = &http.Client{
		Transport: &detiMockRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`stat5.deti-online.com`)),
			},
		},
	}

	info, _, err := s.GetBookInfo(context.Background(), htmlContent, "https://example.com/test")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	cases := []struct {
		expectedAuthor string
	}{
		{authorUnknown},
	}

	for _, c := range cases {
		if info.Author != c.expectedAuthor {
			t.Errorf("expected Author %q, got %q", c.expectedAuthor, info.Author)
		}
	}
}

func TestDetiOnlineGetBookInfo_RequestError(t *testing.T) {
	s := NewDetiOnline()
	htmlContent := `<html><body><script src="http://example.com/audio-player.js"></script></body></html>`
	s.Client = &http.Client{
		Transport: &detiMockRoundTripper{
			err: errors.New("network error"),
		},
	}
	_, _, err := s.GetBookInfo(context.Background(), htmlContent, string([]byte{0x7f}))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestDetiOnlineGetBookInfo_InvalidPlayerURL(t *testing.T) {
	s := NewDetiOnline()
	htmlContent := `<html><body><script src="http://127.0.0.1` + string([]byte{0x7f}) + `/script.js"></script></body></html>`
	_, _, err := s.GetBookInfo(context.Background(), htmlContent, "https://example.com/test")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestDetiOnline_getServerDomain_Errors(t *testing.T) {
	cases := []struct {
		client *http.Client
		name   string
		url    string
	}{
		{
			name: "RequestError",
			url:  "http://\x00",
		},
		{
			client: &http.Client{
				Transport: &detiMockRoundTripper{
					resp: &http.Response{
						StatusCode: http.StatusOK,
						Body:       errCloseReader(0),
					},
				},
			},
			name: "CloseError",
			url:  "http://example.org",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := NewDetiOnline()
			if c.client != nil {
				s.SetClient(c.client)
			}
			fallback := s.getServerDomain(context.Background(), c.url)
			if fallback != "stat4.deti-online.com" {
				t.Fatalf("expected fallback, got %q", fallback)
			}
		})
	}
}

func TestDetiOnlineGetBookInfo_NoStatDomain(t *testing.T) {
	s := NewDetiOnline()
	s.SetClient(&http.Client{
		Transport: &detiMockRoundTripper{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`player.js`)),
			},
		},
	})
	htmlContent := `<html><body><script src="/audio-player.js"></script></body></html>`
	_, _, err := s.GetBookInfo(context.Background(), htmlContent, "https://example.com/test")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestDetiOnlineGetBookInfo_ParseError(t *testing.T) {
	orig := stringsNewReader
	defer func() { stringsNewReader = orig }()

	stringsNewReader = func(_ string) io.Reader {
		return errReader(0)
	}

	s := NewDetiOnline()
	_, _, err := s.GetBookInfo(context.Background(), "html", "https://example.com/test")
	if err == nil || !strings.Contains(err.Error(), "mock read error") {
		t.Fatalf("expected error containing 'mock read error', got %v", err)
	}
}
