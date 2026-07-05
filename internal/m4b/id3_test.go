package m4b

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractID3Text(t *testing.T) {
	tempDir := t.TempDir()

	id3Header := []byte("ID3\x03\x00\x00\x00\x00\x00\x10")
	frameHeader := []byte("TIT2\x00\x00\x00\x06\x00\x00")
	frameData := append([]byte{3}, []byte("Title")...)

	validContent := make([]byte, 0, len(id3Header)+len(frameHeader)+len(frameData))
	validContent = append(validContent, id3Header...)
	validContent = append(validContent, frameHeader...)
	validContent = append(validContent, frameData...)

	validFile := filepath.Join(tempDir, "valid.mp3")
	if err := os.WriteFile(validFile, validContent, 0o600); err != nil {
		t.Fatal(err)
	}

	invalidVerFile := filepath.Join(tempDir, "v5.mp3")
	v5Content := append([]byte("ID3\x05\x00\x00\x00\x00\x00\x10"), frameHeader...)
	v5Content = append(v5Content, frameData...)
	if err := os.WriteFile(invalidVerFile, v5Content, 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		file     string
		frame    string
		expected string
	}{
		{name: "valid file", file: validFile, frame: "TIT2", expected: "Title"},
		{name: "not exist", file: filepath.Join(tempDir, "missing.mp3"), frame: "TPE1", expected: ""},
		{name: "wrong version", file: invalidVerFile, frame: "TRCK", expected: ""},
		{name: "frame not found", file: validFile, frame: "TALB", expected: ""},
		{name: "is a directory", file: tempDir, frame: "COMM", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractID3Text(tt.file, tt.frame); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFindFrame(t *testing.T) {
	header3 := []byte("ID3\x03\x00\x00\x00\x00\x00\x00")
	header4 := []byte("ID3\x04\x00\x00\x00\x00\x00\x00")

	frame1 := append([]byte("TPE1\x00\x00\x00\x04\x00\x00"), []byte("ABCD")...)
	frame2 := append([]byte("TIT2\x00\x00\x00\x05\x00\x00"), []byte("Value")...)

	frame3 := append([]byte("TCON\x00\x00\x00\x05\x00\x00"), []byte("Genre")...)

	data3 := make([]byte, 0, len(header3)+len(frame1)+len(frame2))
	data3 = append(data3, header3...)
	data3 = append(data3, frame1...)
	data3 = append(data3, frame2...)

	data4 := make([]byte, 0, len(header4)+len(frame3))
	data4 = append(data4, header4...)
	data4 = append(data4, frame3...)

	tests := []struct {
		name     string
		frame    string
		expected string
		data     []byte
	}{
		{name: "v2.3 found", data: data3, frame: "TPE1", expected: "ABCD"},
		{name: "v2.4 found", data: data4, frame: "TCON", expected: "Genre"},
		{name: "not found", data: data3, frame: "TALB", expected: ""},
		{name: "truncated", data: data4[:len(data4)-2], frame: "TCON", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := findFrame(tt.data, tt.frame)
			if string(res) != tt.expected {
				t.Errorf("got %q, want %q", string(res), tt.expected)
			}
		})
	}
}

func TestParseTextData(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		data     []byte
	}{
		{name: "empty", expected: "", data: []byte{}},
		{name: "utf16 LE", expected: "Test", data: append([]byte{1, 0xFF, 0xFE}, []byte("T\x00e\x00s\x00t\x00")...)},
		{name: "utf8", expected: "World", data: append([]byte{3}, []byte("World\x00")...)},
		{name: "win1251", expected: "Привет", data: []byte{0, 0xCF, 0xF0, 0xE8, 0xE2, 0xE5, 0xF2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseTextData(tt.data); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDecodeUTF16(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		data     []byte
	}{
		{name: "too short", expected: "", data: []byte{1}},
		{name: "LE BOM", expected: "AB", data: []byte{0xFF, 0xFE, 'A', 0, 'B', 0}},
		{name: "BE BOM", expected: "AB", data: []byte{0xFE, 0xFF, 0, 'A', 0, 'B'}},
		{name: "default BE", expected: "AB", data: []byte{0, 'A', 0, 'B'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeUTF16(tt.data); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPickBestCyrillic(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		data     []byte
	}{
		{name: "valid utf8 cyrillic", expected: "Мир", data: []byte("Мир")},
		{name: "cp866", expected: "Мир", data: []byte{0x8C, 0xA8, 0xE0}},
		{name: "invalid fallback", expected: "яя", data: []byte{0xFF, 0xFF}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickBestCyrillic(tt.data); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestScoreCyrillicString(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		expected int
	}{
		{name: "cyrillic", str: "Тест", expected: 20},
		{name: "ascii", str: "Test", expected: 4},
		{name: "invalid", str: "A\uFFFD", expected: -99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scoreCyrillicString(tt.str); got != tt.expected {
				t.Errorf("got %d, want %d", got, tt.expected)
			}
		})
	}
}
