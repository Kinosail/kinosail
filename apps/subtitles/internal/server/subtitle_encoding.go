package server

import (
	"bytes"
	"errors"
	"io"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	xunicode "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func decodeSubtitleText(data []byte) ([]byte, error) { //nolint:cyclop // One bounded detector rejects ambiguous input before legacy decoding.
	if len(data) == 0 || len(data) > 4<<20 {
		return nil, errors.New("subtitle text is invalid")
	}
	switch {
	case bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}):
		data = data[3:]
	case bytes.HasPrefix(data, []byte{0xff, 0xfe}):
		return subtitleDecodedText(data, xunicode.UTF16(xunicode.LittleEndian, xunicode.ExpectBOM).NewDecoder())
	case bytes.HasPrefix(data, []byte{0xfe, 0xff}):
		return subtitleDecodedText(data, xunicode.UTF16(xunicode.BigEndian, xunicode.ExpectBOM).NewDecoder())
	}
	if len(data) == 0 || bytes.ContainsRune(data, 0) {
		return nil, errors.New("subtitle text is invalid")
	}
	if utf8.Valid(data) {
		return data, nil
	}
	for _, value := range data {
		if value == 0 || value == 0x81 || value == 0x8d || value == 0x8f || value == 0x90 || value == 0x9d {
			return nil, errors.New("subtitle text is invalid")
		}
	}
	return subtitleDecodedText(data, charmap.Windows1252.NewDecoder())
}

func subtitleDecodedText(data []byte, decoder *encoding.Decoder) ([]byte, error) {
	decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), decoder))
	if err != nil || len(decoded) == 0 || len(decoded) > 4<<20 || !utf8.Valid(decoded) || bytes.ContainsRune(decoded, 0) || bytes.ContainsRune(decoded, utf8.RuneError) {
		return nil, errors.New("subtitle text is invalid")
	}
	return decoded, nil
}
