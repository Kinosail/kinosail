package catalog

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/library"
	"golang.org/x/text/unicode/norm"
)

const metadataSearchScratchSize = 512

// Normalize fields into local storage; field separators are normalization boundaries.
func matchesMetadata(item library.Item, query string) bool {
	fields := []string{item.Title, item.Show, item.Year, item.Plot, item.Genres, item.Director, item.Studio, item.Artist, item.Album}
	size := metadataSearchSize(fields, item.Cast, item.ShowCast)
	var storage [metadataSearchScratchSize]byte
	text := storage[:0]
	if size > len(storage) {
		if len(fields[0]) < len(storage) && isASCII(fields[0]) {
			text = appendASCIISearchField(text, fields[0])
			if bytes.Contains(text, []byte(query)) {
				return true
			}
			fields = fields[1:]
		}
		if matchesLiteralPrefix(fields, query) {
			return true
		}
		text = append(make([]byte, 0, size), text...)
	}
	for _, value := range fields {
		text = appendSearchField(text, value)
	}
	for _, people := range [][]library.Person{item.Cast, item.ShowCast} {
		for _, person := range people {
			text = appendSearchField(text, person.Name)
			text = appendSearchField(text, person.Role)
		}
	}
	return bytes.Contains(text, []byte(query))
}

func metadataSearchSize(fields []string, cast, showCast []library.Person) int {
	size := len(fields)
	for _, value := range fields {
		size += len(value)
	}
	for _, people := range [][]library.Person{cast, showCast} {
		for _, person := range people {
			size += len(person.Name) + len(person.Role) + 2
		}
	}
	return size
}

// Lowercase ASCII letters, digits, and spaces survive field normalization unchanged.
func matchesLiteralPrefix(fields []string, query string) bool {
	for position := 0; position < len(query); position++ {
		if searchASCIIByte(query[position]) != query[position] {
			return false
		}
	}
	for _, value := range fields {
		if strings.Contains(value[:min(len(value), metadataSearchScratchSize)], query) {
			return true
		}
	}
	return false
}

func appendSearchField(text []byte, value string) []byte {
	if isASCII(value) {
		return appendASCIISearchField(text, value)
	}
	return appendUnicodeSearchField(text, value)
}

func appendUnicodeSearchField(text []byte, value string) []byte {
	value = norm.NFKD.String(strings.ToLower(value))
	space := true // Every field starts at the preceding field's separator.
	for _, character := range value {
		switch {
		case character >= 0 && character <= 127:
			// Lowercasing precedes NFKD; preserve any capitals produced by decomposition.
			if searchASCIIByte(byte(character)) == ' ' {
				character = ' '
			}
		case unicode.Is(unicode.Mn, character):
			continue
		case !unicode.IsLetter(character) && !unicode.IsDigit(character):
			character = ' '
		}
		if character != ' ' {
			text = utf8.AppendRune(text, character)
			space = false
		} else if !space {
			text = append(text, ' ')
			space = true
		}
	}
	return appendSearchSeparator(text)
}

func appendASCIISearchField(text []byte, value string) []byte {
	for index := 0; index < len(value); index++ {
		character := searchASCIIByte(value[index])
		if character != ' ' || len(text) > 0 && text[len(text)-1] != ' ' {
			text = append(text, character)
		}
	}
	return appendSearchSeparator(text)
}

func appendSearchSeparator(text []byte) []byte {
	if len(text) > 0 && text[len(text)-1] != ' ' {
		text = append(text, ' ')
	}
	return text
}

func searchASCIIByte(character byte) byte {
	if character >= 'A' && character <= 'Z' {
		character += 'a' - 'A'
	}
	if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
		return character
	}
	return ' '
}
