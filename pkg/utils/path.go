// Package utils provides utility functions for the audiobook downloader.
package utils

import (
	"strings"
)

// SanitizeFilename removes or replaces characters that are invalid in filenames
// across various operating systems (Windows, Linux, macOS).
func SanitizeFilename(name string) string {
	invalidChars := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*", "\r", "\n"}

	sanitized := name
	for _, char := range invalidChars {
		sanitized = strings.ReplaceAll(sanitized, char, " ")
	}

	for strings.Contains(sanitized, "  ") {
		sanitized = strings.ReplaceAll(sanitized, "  ", " ")
	}

	return strings.TrimSpace(sanitized)
}
