package gateway

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

type TextDecoder struct{}

func NewTextDecoder() *TextDecoder {
	return &TextDecoder{}
}

func (d *TextDecoder) Decode(input []byte) string {
	if len(input) == 0 {
		return ""
	}

	decoded, err := charmap.Windows1251.NewDecoder().Bytes(input)
	if err != nil {
		return string(input)
	}

	text := string(decoded)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return fixSmaugYaArtifact(text)
}

func fixSmaugYaArtifact(text string) string {
	if text == "" || !strings.ContainsRune(text, 'Я') {
		return text
	}

	runes := []rune(text)
	changed := false

	for i, r := range runes {
		if r != 'Я' {
			continue
		}

		var prev rune
		if i > 0 {
			prev = runes[i-1]
		}

		// Keep legitimate uppercase words like "Ярость", but fix artifact in
		// lowercase/mixed words such as "землЯ" or "вЯлый".
		if isLowerCyrillic(prev) {
			runes[i] = 'я'
			changed = true
		}
	}

	if !changed {
		return text
	}
	return string(runes)
}

func isLowerCyrillic(r rune) bool {
	if r == 0 || r == utf8.RuneError {
		return false
	}
	if r == 'ё' {
		return true
	}
	return r >= 'а' && r <= 'я'
}
