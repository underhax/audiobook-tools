package m4b

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// CleanIntermediateFiles removes downloaded MP3 files and build artifacts (concat and metadata).
func CleanIntermediateFiles(targetDir string) error {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return fmt.Errorf("read target dir: %w", err)
	}

	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(strings.ToLower(e.Name()), ".mp3") || strings.HasSuffix(strings.ToLower(e.Name()), ".m4a")) {
			filePath := filepath.Join(targetDir, e.Name())
			if err := os.Remove(filePath); err != nil {
				if !os.IsNotExist(err) {
					log.Printf("failed to delete intermediate file %s: %v", e.Name(), err)
				}
			}
		}
	}

	extraFiles := []string{"chapters.ffmeta", "ffconcat.txt", "metadata.opf", "cover.jpg"}
	for _, f := range extraFiles {
		filePath := filepath.Join(targetDir, f)
		if err := os.Remove(filePath); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("failed to delete extra file %s: %v", f, err)
			}
		}
	}
	return nil
}
