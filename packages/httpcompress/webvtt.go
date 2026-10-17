package httpcompress

import (
	"compress/gzip"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const (
	MinimumSize = 1024
	level       = 5
)

var writers = sync.Pool{New: func() any {
	writer, _ := gzip.NewWriterLevel(nil, level)
	return writer
}}

func GzipRequested(request *http.Request) bool {
	if request.Method != http.MethodGet {
		return false
	}
	for _, name := range []string{"Range", "If-Range", "If-None-Match", "If-Modified-Since"} {
		if request.Header.Get(name) != "" {
			return false
		}
	}
	accepted, status := Negotiate(request.Header.Values("Accept-Encoding"))
	return accepted && status == 0
}

func WriteWebVTT(writer http.ResponseWriter, request *http.Request, body []byte) {
	Write(writer, request, "text/vtt; charset=utf-8", body)
}

func Write(writer http.ResponseWriter, request *http.Request, contentType string, body []byte) {
	header := writer.Header()
	header.Set("Content-Type", contentType)
	AddVary(header)
	if len(body) < MinimumSize || !GzipRequested(request) {
		header.Set("Content-Length", strconv.Itoa(len(body)))
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(body)
		return
	}
	header.Del("Content-Length")
	header.Set("Content-Encoding", "gzip")
	if tag := header.Get("ETag"); tag != "" && !strings.HasPrefix(tag, "W/") {
		header.Set("ETag", "W/"+tag)
	}
	writer.WriteHeader(http.StatusOK)
	encoder, _ := writers.Get().(*gzip.Writer)
	encoder.Reset(writer)
	_, _ = encoder.Write(body)
	_ = encoder.Close()
	encoder.Reset(nil)
	writers.Put(encoder)
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
