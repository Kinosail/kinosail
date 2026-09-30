package server

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var encodingQuality = regexp.MustCompile(`^(0(\.[0-9]{0,3})?|1(\.0{0,3})?)$`)

var contentCoding = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9a-z-]+$")

// Compress only embedded public text, once when its handler is constructed.
// Personalized HTML, API responses and media never enter this path.
func compressedAsset(content []byte, contentType string) http.HandlerFunc {
	compressed := gzipAsset(content, contentType)
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", contentType)
		cacheStatic(writer, request)
		payload := content
		if len(compressed) != 0 {
			writer.Header().Add("Vary", "Accept-Encoding")
			gzipAccepted, status := assetEncoding(request.Header.Values("Accept-Encoding"))
			if status != 0 {
				writer.Header().Set("Cache-Control", "no-store")
				writer.Header().Set("Content-Length", "0")
				writer.WriteHeader(status)
				return
			}
			if gzipAccepted {
				writer.Header().Set("Content-Encoding", "gzip")
				payload = compressed
			}
		}
		writer.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		if request.Method != http.MethodHead {
			_, _ = writer.Write(payload)
		}
	}
}

func gzipAsset(content []byte, contentType string) []byte {
	var compressed []byte
	if len(content) >= 1024 && (strings.HasPrefix(contentType, "text/") || contentType == "image/svg+xml" || contentType == "application/manifest+json") {
		var output bytes.Buffer
		encoder := gzip.NewWriter(&output)
		_, writeErr := encoder.Write(content)
		if closeErr := encoder.Close(); writeErr == nil && closeErr == nil && output.Len() < len(content) {
			compressed = output.Bytes()
		}
	}
	return compressed
}

func assetEncoding(headers []string) (bool, int) {
	weights, valid := encodingWeights(headers)
	if !valid {
		return false, http.StatusBadRequest
	}
	quality, found := weights["gzip"]
	if !found {
		quality = weights["*"]
	}
	if quality > 0 && quality >= weights["identity"] {
		return true, 0
	}
	identity, explicit := weights["identity"]
	wildcard, wildcardPresent := weights["*"]
	if explicit && identity == 0 || !explicit && wildcardPresent && wildcard == 0 {
		return false, http.StatusNotAcceptable
	}
	return false, 0
}

func encodingWeights(headers []string) (map[string]float64, bool) {
	if len(headers) > 16 {
		return nil, false
	}
	headerBytes := 0
	for _, header := range headers {
		headerBytes += len(header)
	}
	if headerBytes > 4096 {
		return nil, false
	}
	values := strings.Split(strings.Join(headers, ","), ",")
	if len(values) > 16 {
		return nil, false
	}
	weights := make(map[string]float64, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		coding, quality, valid := codingWeight(value)
		if _, duplicate := weights[coding]; duplicate || !valid {
			return nil, false
		}
		weights[coding] = quality
	}
	return weights, true
}

func codingWeight(value string) (string, float64, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(value)), ";")
	coding := strings.TrimSpace(parts[0])
	if len(parts) > 2 || !contentCoding.MatchString(coding) {
		return "", 0, false
	}
	if len(parts) == 1 {
		return coding, 1, true
	}
	name, raw, found := strings.Cut(strings.TrimSpace(parts[1]), "=")
	if !found || name != "q" || !encodingQuality.MatchString(raw) {
		return "", 0, false
	}
	quality, _ := strconv.ParseFloat(raw, 64)
	return coding, quality, true
}
