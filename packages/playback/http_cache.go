package playback

import (
	"io"
	"net/http"
)

const (
	ArtworkCacheControl  = "private, max-age=86400"
	HLSMediaCacheControl = "private, max-age=3600"
)

type cachedResponseWriter struct {
	http.ResponseWriter
	policy  string
	written bool
}

func CacheSuccessfulResponses(writer http.ResponseWriter, policy string) http.ResponseWriter {
	return &cachedResponseWriter{ResponseWriter: writer, policy: policy}
}

func CacheSuccessful(policy string, next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		next(CacheSuccessfulResponses(writer, policy), request)
	}
}

func (writer *cachedResponseWriter) WriteHeader(status int) {
	if !writer.written {
		writer.written = true
		value := "no-store"
		if status == http.StatusOK || status == http.StatusPartialContent || status == http.StatusNotModified {
			value = writer.policy
		}
		writer.Header().Set("Cache-Control", value)
	}
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *cachedResponseWriter) Write(data []byte) (int, error) {
	if !writer.written {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(data)
}

func (writer *cachedResponseWriter) ReadFrom(source io.Reader) (int64, error) {
	if !writer.written {
		writer.WriteHeader(http.StatusOK)
	}
	if destination, ok := writer.ResponseWriter.(io.ReaderFrom); ok {
		return destination.ReadFrom(source)
	}
	return io.Copy(writer.ResponseWriter, source)
}

func (writer *cachedResponseWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }
