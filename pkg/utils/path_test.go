package utils

import (
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid filename",
			input:    "Book Title 123",
			expected: "Book Title 123",
		},
		{
			name:     "invalid characters",
			input:    "Book: Title <Part 1> / \\ | ? * \"",
			expected: "Book Title Part 1",
		},
		{
			name:     "multiple spaces",
			input:    "Book   Title    1",
			expected: "Book Title 1",
		},
		{
			name:     "newlines",
			input:    "Book\nTitle\r1",
			expected: "Book Title 1",
		},
		{
			name:     "leading and trailing spaces",
			input:    "  Book Title  ",
			expected: "Book Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeFilename(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
