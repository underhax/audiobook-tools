package core

import (
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
		})
	}
}
