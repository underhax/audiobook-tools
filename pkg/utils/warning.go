package utils

import (
	"fmt"
	"io"
)

// PrintUnfinishedWarning writes a formatted warning about unfinished downloads to w.
func PrintUnfinishedWarning(w io.Writer, files []string, suffix string) error {
	if _, err := fmt.Fprintf(w, "Warning: Unfinished downloads:\n"); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	for _, f := range files {
		if _, err := fmt.Fprintf(w, "  • %s\n", f); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}
	if suffix != "" {
		if _, err := fmt.Fprintf(w, "%s\n", suffix); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}
	return nil
}
