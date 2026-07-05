// Package core contains the core data structures and logic for the downloader.
package core

import "strings"

// BookInfo represents metadata about an audiobook.
type BookInfo struct {
	PublishedYear  string   `json:"publishedYear"`
	Publisher      string   `json:"publisher,omitempty"`
	Author         string   `json:"author"`
	Narrator       string   `json:"narrator"`
	Cover          string   `json:"cover"`
	Description    string   `json:"description"`
	Title          string   `json:"title"`
	AgeRestriction string   `json:"ageRestriction,omitempty"`
	URL            string   `json:"url"`
	Language       string   `json:"language,omitempty"`
	Series         string   `json:"series,omitempty"`
	Authors        []string `json:"authors"`
	Genres         []string `json:"genres"`
	Translators    []string `json:"translators,omitempty"`
	Narrators      []string `json:"narrators"`
}

// Chapter represents a single downloadable audio file.
type Chapter struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Extension string `json:"extension,omitempty"`
}

// AuthError represents an authentication error for a specific provider.
type AuthError struct {
	ProviderName string
	ProviderID   string
	EnvVar       string
}

// Error implements the error interface.
func (e *AuthError) Error() string {
	return "authentication required"
}

// FormattedDescription returns the description with appended translators and age restriction localized based on language.
func (b *BookInfo) FormattedDescription() string {
	desc := b.Description
	isRU := b.Language == "" || strings.HasPrefix(strings.ToLower(b.Language), "ru")

	if len(b.Translators) > 0 {
		transLabel := "Translator"
		if isRU {
			transLabel = "Переводчик"
		}
		desc += "\n\n" + transLabel + ": " + strings.Join(b.Translators, ", ")
	}

	if b.AgeRestriction != "" {
		ageLabel := "Age rating"
		if isRU {
			ageLabel = "Возрастное ограничение"
		}
		if len(b.Translators) == 0 {
			desc += "\n\n"
		} else {
			desc += "\n"
		}
		desc += ageLabel + ": " + b.AgeRestriction + "+"
	}

	return desc
}
