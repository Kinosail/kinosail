package webassets

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/MikeO7/kinosail/packages/httpcompress"
)

const (
	immutableCache  = "public, max-age=31536000, immutable"
	revalidateCache = "public, max-age=86400"
)

func Version(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func Static(content []byte, contentType string) http.HandlerFunc {
	version := sync.OnceValue(func() string { return Version(content) })
	compressed := sync.OnceValue(func() []byte { return precompress(content, contentType) })
	return func(writer http.ResponseWriter, request *http.Request) {
		header := writer.Header()
		current := version()
		header.Set("Cache-Control", revalidateCache)
		if request.URL.Query().Get("v") == current {
			header.Set("Cache-Control", immutableCache)
		}
		payload, tag := content, `"`+current+`"`
		if gzipped := compressed(); len(gzipped) != 0 {
			httpcompress.AddVary(header)
			accepted, status := httpcompress.Negotiate(request.Header.Values("Accept-Encoding"))
			if status != 0 {
				header.Set("Cache-Control", "no-store")
				header.Set("Content-Length", "0")
				writer.WriteHeader(status)
				return
			}
			if accepted {
				header.Set("Content-Encoding", "gzip")
				payload, tag = gzipped, `"`+current+`-gzip"`
			}
		}
		header.Set("ETag", tag)
		if unchanged(request.Header.Values("If-None-Match"), current) {
			writer.WriteHeader(http.StatusNotModified)
			return
		}
		header.Set("Content-Type", contentType)
		header.Set("Content-Length", strconv.Itoa(len(payload)))
		if request.Method != http.MethodHead {
			_, _ = writer.Write(payload)
		}
	}
}

func precompress(content []byte, contentType string) []byte {
	if len(content) < httpcompress.MinimumSize || !strings.HasPrefix(contentType, "text/") && contentType != "image/svg+xml" && contentType != "application/manifest+json" {
		return nil
	}
	var output bytes.Buffer
	encoder, _ := gzip.NewWriterLevel(&output, gzip.BestCompression)
	_, writeErr := encoder.Write(content)
	if closeErr := encoder.Close(); writeErr != nil || closeErr != nil || output.Len() >= len(content) {
		return nil
	}
	return output.Bytes()
}

func unchanged(conditions []string, version string) bool {
	if len(conditions) > 16 {
		return false
	}
	for _, condition := range conditions {
		for _, tag := range strings.Split(condition, ",") {
			tag = strings.TrimPrefix(strings.TrimSpace(tag), "W/")
			if tag == "*" || tag == `"`+version+`"` || tag == `"`+version+`-gzip"` {
				return true
			}
		}
	}
	return false
}
