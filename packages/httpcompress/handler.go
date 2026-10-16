package httpcompress

import (
	"compress/gzip"
	"mime"
	"net/http"
	"strings"
	"sync"
)

const (
	MinimumSize = 1024
	level       = 5
)

var compressibleTypes = map[string]bool{
	"text/html": true, "text/css": true, "text/javascript": true, "application/javascript": true,
	"application/json": true, "text/vtt": true,
}

var writers = sync.Pool{New: func() any {
	writer, _ := gzip.NewWriterLevel(nil, level)
	return writer
}}

func Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			next.ServeHTTP(writer, request)
			return
		}
		wrapped := &responseWriter{ResponseWriter: writer, allowed: compressionAllowed(request)}
		defer wrapped.finish()
		next.ServeHTTP(wrapped, request)
	})
}

func compressionAllowed(request *http.Request) bool {
	if request.Method != http.MethodGet || request.Header.Get("Range") != "" {
		return false
	}
	if accepted, status := Negotiate(request.Header.Values("Accept-Encoding")); !accepted || status != 0 {
		return false
	}
	switch request.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return true
	case "":
		return request.Header.Get("Cookie") == ""
	}
	return false
}

func Compressible(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && compressibleTypes[mediaType]
}

func AddVary(header http.Header) {
	for _, value := range header.Values("Vary") {
		for _, token := range strings.Split(value, ",") {
			if token = strings.TrimSpace(token); token == "*" || strings.EqualFold(token, "Accept-Encoding") {
				return
			}
		}
	}
	header.Add("Vary", "Accept-Encoding")
}

func weakenETag(header http.Header) {
	if tag := header.Get("ETag"); tag != "" && !strings.HasPrefix(tag, "W/") {
		header.Set("ETag", "W/"+tag)
	}
}
