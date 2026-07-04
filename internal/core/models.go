// Package core contains the core data structures and logic for the downloader.
package core

// BookInfo represents metadata about an audiobook.
type BookInfo struct {
	URL           string   `json:"url"`
	Title         string   `json:"title"`
	Author        string   `json:"author"`
	Narrator      string   `json:"narrator"`
	Cover         string   `json:"cover"`
	Description   string   `json:"description"`
	PublishedYear string   `json:"publishedYear"`
	Authors       []string `json:"authors"`
	Narrators     []string `json:"narrators"`
	Genres        []string `json:"genres"`
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
