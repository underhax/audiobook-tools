package m4b

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// ExtractID3Text reads a specific ID3v2 text frame from the file header.
func ExtractID3Text(path, frameID string) string {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return ""
	}
	defer func() {
		closeErr := f.Close()
		_ = closeErr
	}()

	data := make([]byte, 65536)
	n, err := io.ReadFull(f, data)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return ""
	}
	data = data[:n]

	if len(data) < 10 || string(data[:3]) != "ID3" {
		return ""
	}

	version := data[3]
	if version != 3 && version != 4 {
		return ""
	}

	frameData := findFrame(data, frameID)
	if frameData == nil {
		return ""
	}

	return parseTextData(frameData)
}

func findFrame(data []byte, frameID string) []byte {
	i := 10
	for i+10 < len(data) {
		id := string(data[i : i+4])
		var frameSize int
		if data[3] == 4 {
			frameSize = int(data[i+4])<<21 | int(data[i+5])<<14 | int(data[i+6])<<7 | int(data[i+7])
		} else {
			frameSize = int(data[i+4])<<24 | int(data[i+5])<<16 | int(data[i+6])<<8 | int(data[i+7])
		}

		if id == frameID {
			if i+10+frameSize > len(data) {
				return nil
			}
			return data[i+10 : i+10+frameSize]
		}
		i += 10 + frameSize
	}

	return nil
}

func parseTextData(b []byte) string {
	if len(b) < 1 {
		return ""
	}

	encodingByte := b[0]
	textBytes := b[1:]

	if encodingByte == 1 || encodingByte == 2 {
		return decodeUTF16(textBytes)
	}

	textBytes = bytes.TrimRight(textBytes, "\x00")

	if encodingByte == 3 && utf8.Valid(textBytes) {
		return string(textBytes)
	}

	return pickBestCyrillic(textBytes)
}

func decodeUTF16(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	var order binary.ByteOrder
	switch {
	case b[0] == 0xFE && b[1] == 0xFF:
		order = binary.BigEndian
		b = b[2:]
	case b[0] == 0xFF && b[1] == 0xFE:
		order = binary.LittleEndian
		b = b[2:]
	default:
		order = binary.BigEndian
	}

	var runes []rune
	for i := 0; i+1 < len(b); i += 2 {
		u := order.Uint16(b[i : i+2])
		runes = append(runes, rune(u))
	}
	s := string(runes)
	return strings.TrimSpace(strings.TrimRight(s, "\x00"))
}

func pickBestCyrillic(b []byte) string {
	if utf8.Valid(b) {
		s := string(b)
		if scoreCyrillicString(s) > 0 {
			return strings.TrimSpace(s)
		}
	}

	bestScore := -1
	bestStr := string(b)

	encodings := []*charmap.Charmap{
		charmap.Windows1251,
		charmap.KOI8R,
		charmap.CodePage866,
		charmap.ISO8859_5,
	}

	for _, enc := range encodings {
		dec := enc.NewDecoder()
		res, err := dec.Bytes(b)
		if err != nil {
			continue
		}

		str := string(res)
		score := scoreCyrillicString(str)

		if score > bestScore {
			bestScore = score
			bestStr = str
		}
	}

	return strings.TrimSpace(strings.TrimRight(bestStr, "\x00"))
}

func scoreCyrillicString(str string) int {
	score := 0
	for _, r := range str {
		switch {
		case (r >= 'А' && r <= 'Я') || (r >= 'а' && r <= 'я') || r == 'Ё' || r == 'ё':
			score += 5
		case r >= 32 && r <= 126:
			score++
		case r == '\uFFFD':
			score -= 100
		}
	}
	return score
}
