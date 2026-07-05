package core

import (
	"testing"
)

func TestAuthError(t *testing.T) {
	err := &AuthError{
		ProviderName: "Test Provider",
		ProviderID:   "test_provider",
		EnvVar:       "TEST_ENV",
	}

	if err.Error() != "authentication required" {
		t.Errorf("expected 'authentication required', got %q", err.Error())
	}
}

func TestFormattedDescription(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		info     BookInfo
	}{
		{
			name:     "no language, no translators, no age restriction",
			expected: "Test Description A",
			info: BookInfo{
				Description: "Test Description A",
			},
		},
		{
			name:     "ru language, with translators and age restriction",
			expected: "Test Description B\n\nПереводчик: Иван, Петр\nВозрастное ограничение: 16+",
			info: BookInfo{
				Description:    "Test Description B",
				Language:       "ru",
				Translators:    []string{"Иван", "Петр"},
				AgeRestriction: "16",
			},
		},
		{
			name:     "en language, with translators and age restriction",
			expected: "Test Description C\n\nTranslator: John, Peter\nAge rating: 18+",
			info: BookInfo{
				Description:    "Test Description C",
				Language:       "en",
				Translators:    []string{"John", "Peter"},
				AgeRestriction: "18",
			},
		},
		{
			name:     "ru language, only age restriction",
			expected: "Test Description D\n\nВозрастное ограничение: 12+",
			info: BookInfo{
				Description:    "Test Description D",
				Language:       "ru",
				AgeRestriction: "12",
			},
		},
		{
			name:     "en language, only age restriction",
			expected: "Test Description E\n\nAge rating: 6+",
			info: BookInfo{
				Description:    "Test Description E",
				Language:       "en",
				AgeRestriction: "6",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.info.FormattedDescription()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
