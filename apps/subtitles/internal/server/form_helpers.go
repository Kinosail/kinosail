package server

import (
	"mime"
	"net/http"
	"net/url"
	"slices"
)

func oneValue(values url.Values, key string, limit int) (string, bool) {
	return firstBounded(values[key], limit, true)
}

func optionalValue(values url.Values, key string, limit int) (string, bool) {
	value, found := values[key]
	if !found {
		return "", true
	}
	return firstBounded(value, limit, false)
}

func firstBounded(values []string, limit int, required bool) (string, bool) {
	value := ""
	if len(values) == 1 {
		value = values[0]
	}
	return value, len(values) == 1 && len(value) <= limit && (!required || value != "")
}

func onlyFormKeys(form url.Values, keys ...string) bool {
	for key := range form {
		if !slices.Contains(keys, key) {
			return false
		}
	}
	return true
}

func formEncoded(request *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/x-www-form-urlencoded"
}
