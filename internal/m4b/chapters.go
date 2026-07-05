package m4b

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/underhax/audiobook-tools/internal/core"
)

// ExtractChaptersFromDir scans a directory for mp3 files and generates a chapter list.
func ExtractChaptersFromDir(dir string) ([]core.Chapter, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var audioFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		lower := strings.ToLower(entry.Name())
		if strings.HasSuffix(lower, ".mp3") || strings.HasSuffix(lower, ".m4a") {
			audioFiles = append(audioFiles, entry.Name())
		}
	}

	sort.Strings(audioFiles)

	var chapters []core.Chapter
	for _, file := range audioFiles {
		filePath := dir + "/" + file
		name := ExtractID3Text(filePath, "TIT2")

		if name == "" {
			name = file
			if strings.HasSuffix(strings.ToLower(name), ".mp3") {
				name = name[:len(name)-4]
			} else if strings.HasSuffix(strings.ToLower(name), ".m4a") {
				name = name[:len(name)-4]
			}
			parts := strings.SplitN(name, " ", 2)
			if len(parts) == 2 && len(parts[0]) == 3 {
				name = parts[1]
			}
		}

		chapters = append(chapters, core.Chapter{
			Title: name,
			URL:   "",
		})
	}

	if len(chapters) == 0 {
		return nil, errors.New("no source audio files (.mp3 or .m4a) found in directory")
	}

	return chapters, nil
}
