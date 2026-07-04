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

	var mp3Files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".mp3") {
			mp3Files = append(mp3Files, entry.Name())
		}
	}

	sort.Strings(mp3Files)

	var chapters []core.Chapter
	for _, file := range mp3Files {
		filePath := dir + "/" + file
		name := ExtractID3Text(filePath, "TIT2")

		if name == "" {
			name = strings.TrimSuffix(file, ".mp3")
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
		return nil, errors.New("no mp3 files found in directory")
	}

	return chapters, nil
}
