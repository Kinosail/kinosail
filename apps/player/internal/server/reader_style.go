package server

import (
	"bytes"
	_ "embed"
	"regexp"
	"strings"
)

//go:embed static/reader-chapter.css
var readerChapterCSS []byte

var readerChapterAnchors = []*regexp.Regexp{regexp.MustCompile(`(?i)<head(\s[^>]*)?>`), regexp.MustCompile(`(?i)<html(\s[^>]*)?>`)}

func readerChapterType(contentType string) bool {
	mediaType, _, _ := strings.Cut(contentType, ";")
	return mediaType == "text/html" || mediaType == "application/xhtml+xml"
}

func styledReaderChapter(data []byte) []byte {
	style := bytes.Join([][]byte{[]byte("<style>"), readerChapterCSS, []byte("</style>")}, nil)
	for _, anchor := range readerChapterAnchors {
		if location := anchor.FindIndex(data); location != nil {
			return bytes.Join([][]byte{data[:location[1]], style, data[location[1]:]}, nil)
		}
	}
	return data
}
