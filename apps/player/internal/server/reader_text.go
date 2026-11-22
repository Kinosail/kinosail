package server

import (
	"bytes"
	"errors"
	"html"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	textPartBytes     = 256 << 10
	textPartLookahead = 16 << 10
	textBookLimit     = 64 << 20
)

var (
	textPartName      = regexp.MustCompile(`^part-([1-9][0-9]{0,3})\.html$`)
	errTextPart       = errors.New("invalid text part")
	errTextBookLength = errors.New("text book too large")
	textLineBreaks    = strings.NewReplacer("\r\n", "\n", "\r", "\n")
)

func textPartCount(size int64) int {
	if size <= 0 || size > textBookLimit {
		return 0
	}
	return int((size + textPartBytes - 1) / textPartBytes)
}

func textPages(request *http.Request, item library.Item) []readerPage {
	info, err := os.Lstat(item.Path)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	count := textPartCount(info.Size())
	if count == 0 && info.Size() > textBookLimit {
		logTextPart(request, item, "too_large")
	}
	pages := make([]readerPage, count)
	for index := range pages {
		pages[index] = readerPage{Title: "Part " + strconv.Itoa(index+1), URL: "/read/" + item.ID + "/asset/part-" + strconv.Itoa(index+1) + ".html"}
	}
	return pages
}

func serveTextPart(writer http.ResponseWriter, request *http.Request, item library.Item) {
	data, err := textPart(item, request.PathValue("asset"))
	if err != nil {
		logTextPart(request, item, textPartFailure(err))
		localizedNotFound(writer, request)
		return
	}
	readerAssetHeaders(writer)
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = writer.Write(styledReaderChapter(data))
}

func logTextPart(request *http.Request, item library.Item, outcome string) {
	slog.WarnContext(request.Context(), "reader text part rejected", "operation", "reader.text_part", "request_id", requestActivityID(request.Context()), "item_id", item.ID, "outcome", outcome)
}

func textPartFailure(err error) string {
	switch {
	case errors.Is(err, errTextPart):
		return "invalid_part"
	case errors.Is(err, errTextBookLength):
		return "too_large"
	case errors.Is(err, fs.ErrNotExist):
		return "missing"
	case errors.Is(err, fs.ErrPermission):
		return "permission"
	default:
		return "unreadable"
	}
}

func textPart(item library.Item, name string) ([]byte, error) {
	match := textPartName.FindStringSubmatch(name)
	if match == nil {
		return nil, errTextPart
	}
	number, _ := strconv.Atoi(match[1])
	info, err := os.Lstat(item.Path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fs.ErrNotExist
	}
	file, err := os.Open(item.Path) // #nosec G304 -- regular file beneath a configured Library root.
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	size := opened.Size()
	count := textPartCount(size)
	if size > textBookLimit {
		return nil, errTextBookLength
	}
	if number > count {
		return nil, errTextPart
	}
	start, err := textBoundary(file, size, int64(number-1)*textPartBytes)
	if err != nil {
		return nil, err
	}
	end, err := textBoundary(file, size, int64(number)*textPartBytes)
	if err != nil {
		return nil, err
	}
	raw := make([]byte, end-start)
	if _, err := file.ReadAt(raw, start); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if number == 1 {
		raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	}
	return textDocument(item.Title, raw), nil
}

func textBoundary(file *os.File, size, offset int64) (int64, error) {
	if offset <= 0 {
		return 0, nil
	}
	if offset >= size {
		return size, nil
	}
	window := make([]byte, min(textPartLookahead, size-offset))
	if _, err := file.ReadAt(window, offset); err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	if newline := bytes.IndexByte(window, '\n'); newline >= 0 {
		return offset + int64(newline) + 1, nil
	}
	skip := 0
	for skip < len(window) && skip < utf8.UTFMax && !utf8.RuneStart(window[skip]) {
		skip++
	}
	return offset + int64(skip), nil
}

func textDocument(title string, raw []byte) []byte {
	text := textLineBreaks.Replace(strings.ToValidUTF8(string(raw), "�"))
	text = strings.Map(func(character rune) rune {
		if character == '\n' || character == '\t' {
			return character
		}
		if unicode.IsControl(character) || character == '￾' || character == '￿' {
			return -1
		}
		return character
	}, text)
	var document strings.Builder
	document.Grow(len(text) + len(text)/8 + 256)
	document.WriteString(`<!doctype html><html><head><meta charset="utf-8"><title>`)
	document.WriteString(html.EscapeString(title))
	document.WriteString(`</title></head><body>`)
	for _, paragraph := range textParagraphs(text) {
		document.WriteString("<p>")
		for index, line := range paragraph {
			if index > 0 {
				document.WriteString("<br>")
			}
			document.WriteString(html.EscapeString(line))
		}
		document.WriteString("</p>")
	}
	document.WriteString(`</body></html>`)
	return []byte(document.String())
}

func textParagraphs(text string) [][]string {
	lines := strings.Split(text, "\n")
	blocks := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blocks = true
			break
		}
	}
	paragraphs := make([][]string, 0, len(lines)/4+1)
	var current []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if len(current) > 0 {
				paragraphs, current = append(paragraphs, current), nil
			}
			continue
		}
		if !blocks {
			paragraphs = append(paragraphs, []string{line})
			continue
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		paragraphs = append(paragraphs, current)
	}
	return paragraphs
}

func readerAssetHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Content-Security-Policy", "sandbox allow-same-origin; default-src 'self' data:; script-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'self'")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("X-Frame-Options", "SAMEORIGIN")
}
