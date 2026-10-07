package server_test

import (
	"net/http"
	"net/url"
	"strings"
)

// Parse only the already-decoded request path for cache evidence. The actual
// HTTP request, including its playlist-carried session, is never rewritten.
func speedFailureRequestTarget(request *http.Request) string {
	if request == nil || request.URL == nil {
		return ""
	}
	value := request.URL
	if !speedFailureRequestURL(value) || !speedFailureRequestQuery(value.RawQuery) {
		return ""
	}
	target := speedFailureRequestPath(value.Path)
	if !speedFailureTarget(target) {
		return ""
	}
	return target
}

func speedFailureRequestURL(value *url.URL) bool {
	if len(value.Path) > 2048 || len(value.RawQuery) > 256 {
		return false
	}
	if value.Scheme != "" || value.Host != "" || value.User != nil || value.Opaque != "" {
		return false
	}
	return value.Fragment == "" && !value.ForceQuery && value.RawPath == ""
}

func speedFailureRequestPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[0] != "" {
		return ""
	}
	for _, part := range parts[1:] {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "\\\x00\r\n") {
			return ""
		}
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

func speedFailureRequestQuery(raw string) bool {
	if raw == "" {
		return true
	}
	if strings.Contains(raw, "&&") || strings.HasPrefix(raw, "&") || strings.HasSuffix(raw, "&") {
		return false
	}
	values, err := url.ParseQuery(raw)
	if err != nil || len(values) != 1 || len(values["playbackSession"]) != 1 {
		return false
	}
	session := values["playbackSession"][0]
	return len(session) >= 8 && len(session) <= 64 && strings.Trim(session, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") == ""
}
