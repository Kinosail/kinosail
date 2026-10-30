package server

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"golang.org/x/text/encoding/charmap"
)

func legacySubtitle(t *testing.T, line string, encoder *charmap.Charmap) []byte {
	t.Helper()
	encoded, err := encoder.NewEncoder().Bytes([]byte("1\r\n00:00:01,000 --> 00:00:02,500\r\n" + line + "\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestLegacyCodepageSubtitlesPassDownloadValidationAndDecodeByLanguage(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		language, text string
		encoder        *charmap.Charmap
		marker         byte
	}{
		{"cs", "Ťuk na dveře", charmap.Windows1250, 0x8d},
		{"fa", "پچگ سلام", charmap.Windows1256, 0x81},
		{"ru", "Привет", charmap.Windows1251, 0xcf},
	} {
		data := legacySubtitle(t, test.text, test.encoder)
		if !bytes.ContainsRune(data, rune(test.marker)) && !bytes.Contains(data, []byte{test.marker}) {
			t.Fatalf("%s fixture does not contain byte %#x", test.language, test.marker)
		}
		if !validSubtitlePayload(data) {
			t.Fatalf("%s legacy subtitle rejected at download validation", test.language)
		}
		converted, err := convertSubtitle(data, test.language, subtitleConversionOptions{})
		if err != nil || !strings.Contains(string(converted.Data), test.text) {
			t.Fatalf("%s conversion = %q, %v", test.language, converted.Data, err)
		}
	}
	for _, invalid := range [][]byte{[]byte("not a subtitle"), {0x00, 0x2d, 0x2d, 0x3e}, nil} {
		if validSubtitlePayload(invalid) {
			t.Fatalf("invalid payload %q accepted", invalid)
		}
	}
}

func TestSubSourceArchiveAcceptsALegacyCodepageSubtitle(t *testing.T) {
	t.Parallel()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.Create("Film.cs.srt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(legacySubtitle(t, "Ťuk na dveře", charmap.Windows1250)); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if data, err := readSubSourceArchive(archive, library.Item{Kind: "video"}); err != nil || len(data) == 0 {
		t.Fatalf("SubSource legacy archive = %d bytes, %v", len(data), err)
	}
}
