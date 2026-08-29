package core

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateOPF(t *testing.T) {
	tests := []struct {
		name    string
		info    BookInfo
		wantErr bool
	}{
		{
			name: "full info",
			info: BookInfo{
				Title:         "Title <with> Tags",
				Author:        "John & Doe",
				Narrator:      "Jane Doe",
				Description:   "A great book.",
				PublishedYear: "2023",
				Series:        "My Series",
				SeriesNumber:  "4",
				Genres:        []string{"Sci-Fi", "Fiction & Fantasy"},
			},
			wantErr: false,
		},
		{
			name: "missing narrator and year",
			info: BookInfo{
				Title:       "Simple Book",
				Author:      "Author",
				Description: "Desc",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xmlStr, err := GenerateOPF(&tt.info)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GenerateOPF error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if tt.info.Narrator != "" && !strings.Contains(xmlStr, "Jane Doe") {
				t.Errorf("Expected narrator")
			}
			if tt.info.Narrator == "" && strings.Contains(xmlStr, "opf:role=\"nrt\"") {
				t.Errorf("Did not expect narrator")
			}
			if tt.info.SeriesNumber != "" && !strings.Contains(xmlStr, "calibre:series_index") {
				t.Errorf("Expected series index")
			}
		})
	}
}

type errorWriter struct{}

func (e errorWriter) Write(_ []byte) (n int, err error) {
	return 0, errors.New("writer error")
}

func TestDefaultExecuteTemplate_Error(t *testing.T) {
	err := defaultExecuteTemplate(errorWriter{}, BookInfo{})
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func mockExecuteTemplateError(_ io.Writer, _ any) error {
	return errors.New("mock template error")
}

func TestParseOPF_Valid(t *testing.T) {
	tempDir := t.TempDir()

	validOPF := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf">
  <metadata>
    <title>Test Title</title>
    <description>Test Desc</description>
    <date>2024</date>
    <publisher>Test Pub</publisher>
    <language>rus</language>
    <creator xmlns:opf="http://www.idpf.org/2007/opf" opf:role="aut">Author Name</creator>
    <creator xmlns:opf="http://www.idpf.org/2007/opf" opf:role="nrt">Narrator Name</creator>
    <subject>Fiction</subject>
    <subject>   </subject>
    <meta name="calibre:series" content="Test Series"/>
    <meta name="calibre:series_index" content="4"/>
  </metadata>
</package>`
	validPath := filepath.Join(tempDir, "valid.opf")
	if writeErr := os.WriteFile(validPath, []byte(validOPF), 0o600); writeErr != nil {
		t.Fatalf("write valid opf: %v", writeErr)
	}

	info, err := ParseOPF(validPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if info.Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got '%s'", info.Title)
	}
	if info.Author != "Author Name" {
		t.Errorf("expected author 'Author Name', got '%s'", info.Author)
	}
	if info.Narrator != "Narrator Name" {
		t.Errorf("expected narrator 'Narrator Name', got '%s'", info.Narrator)
	}
	if info.Publisher != "Test Pub" {
		t.Errorf("expected publisher 'Test Pub', got '%s'", info.Publisher)
	}
	if info.Series != "Test Series" {
		t.Errorf("expected series 'Test Series', got '%s'", info.Series)
	}
	if info.SeriesNumber != "4" {
		t.Errorf("expected series number '4', got '%s'", info.SeriesNumber)
	}
	if info.Language != "rus" {
		t.Errorf("expected language 'rus', got '%s'", info.Language)
	}
	if len(info.Genres) != 1 || info.Genres[0] != "Fiction" {
		t.Errorf("expected genres ['Fiction'], got %v", info.Genres)
	}
}

func TestParseOPF_AlternativeSeriesAndFallback(t *testing.T) {
	tempDir := t.TempDir()

	altSeriesOPF := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf">
  <metadata>
    <meta name="series" content="Alt Series"/>
    <meta name="series_index" content="5"/>
    <meta property="belongs-to-collection">Collection Series</meta>
    <meta property="group-position">7</meta>
  </metadata>
</package>`
	altPath := filepath.Join(tempDir, "alt_series.opf")
	if writeErr := os.WriteFile(altPath, []byte(altSeriesOPF), 0o600); writeErr != nil {
		t.Fatalf("write alt series opf: %v", writeErr)
	}
	infoAlt, err := ParseOPF(altPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if infoAlt.Series != "Collection Series" {
		t.Errorf("expected series 'Collection Series', got '%s'", infoAlt.Series)
	}
	if infoAlt.SeriesNumber != "7" {
		t.Errorf("expected series number '7', got '%s'", infoAlt.SeriesNumber)
	}

	fallbackOPF := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf">
  <metadata>
    <creator xmlns:opf="http://www.idpf.org/2007/opf" opf:role="unknown">Fallback Author</creator>
  </metadata>
</package>`
	fallbackPath := filepath.Join(tempDir, "fallback.opf")
	if writeErr := os.WriteFile(fallbackPath, []byte(fallbackOPF), 0o600); writeErr != nil {
		t.Fatalf("write fallback opf: %v", writeErr)
	}
	infoFallback, err := ParseOPF(fallbackPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if infoFallback.Author != "Fallback Author" {
		t.Errorf("expected fallback author 'Fallback Author', got '%s'", infoFallback.Author)
	}
}

func TestParseOPF_Errors(t *testing.T) {
	tempDir := t.TempDir()

	_, err := ParseOPF(filepath.Join(tempDir, "missing.opf"))
	if err == nil {
		t.Errorf("expected error for missing file")
	}

	invalidPath := filepath.Join(tempDir, "invalid.opf")
	if writeErr := os.WriteFile(invalidPath, []byte("<broken>"), 0o600); writeErr != nil {
		t.Fatalf("write invalid opf: %v", writeErr)
	}
	_, err = ParseOPF(invalidPath)
	if err == nil {
		t.Errorf("expected error for invalid xml")
	}
}

func TestGenerateOPF_Error(t *testing.T) {
	oldExecute := executeTemplate
	executeTemplate = mockExecuteTemplateError
	defer func() { executeTemplate = oldExecute }()

	_, err := GenerateOPF(&BookInfo{})
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}
