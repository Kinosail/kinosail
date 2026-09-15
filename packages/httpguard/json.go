package httpguard

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"slices"
)

// RequiredValue returns one non-empty bounded form value.
func RequiredValue(values url.Values, key string, limit int) (string, bool) {
	return BoundedValue(values[key], limit, true)
}

// OptionalValue returns one bounded value or accepts an omitted form key.
func OptionalValue(values url.Values, key string, limit int) (string, bool) {
	value, found := values[key]
	if !found {
		return "", true
	}
	return BoundedValue(value, limit, false)
}

// BoundedValue accepts exactly one value within limit, optionally requiring content.
func BoundedValue(values []string, limit int, required bool) (string, bool) {
	value := ""
	if len(values) == 1 {
		value = values[0]
	}
	return value, len(values) == 1 && len(value) <= limit && (!required || value != "")
}

// OnlyFormKeys rejects form fields outside the supplied allowlist.
func OnlyFormKeys(form url.Values, keys ...string) bool {
	for key := range form {
		if !slices.Contains(keys, key) {
			return false
		}
	}
	return true
}

// FormEncoded reports whether a request has the URL-encoded form media type.
func FormEncoded(request *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/x-www-form-urlencoded"
}

// DecodeJSON decodes one bounded external JSON document.
func DecodeJSON(reader io.Reader, maximum int64, target any, strict bool) error {
	data, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil || int64(len(data)) > maximum {
		return errors.New("external JSON response is too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if strict {
		decoder.DisallowUnknownFields()
	}
	if decoder.Decode(target) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("external JSON response is invalid")
	}
	return nil
}

// DecodeRequestJSON decodes one strict, one-megabyte HTTP JSON object.
func DecodeRequestJSON(writer http.ResponseWriter, request *http.Request, target any) error {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid JSON request")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON object")
	}
	return nil
}

// DecodeForm parses one bounded form with no query, unknown, or repeated values.
func DecodeForm(writer http.ResponseWriter, request *http.Request, maximum int64, keys ...string) error {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" || request.URL.RawQuery != "" {
		return errors.New("invalid form request")
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maximum)
	if request.ParseForm() != nil {
		return errors.New("invalid form request")
	}
	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
	}
	for key, values := range request.PostForm {
		if _, ok := allowed[key]; !ok || len(values) != 1 {
			return errors.New("invalid form request")
		}
	}
	return nil
}

// EmptyMutationRequest accepts only an empty, bounded body and empty query.
func EmptyMutationRequest(writer http.ResponseWriter, request *http.Request) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, 1024)
	mediaType, _, mediaErr := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if mediaErr == nil && mediaType == "application/x-www-form-urlencoded" {
		return request.ParseForm() == nil && len(request.PostForm) == 0 && request.URL.RawQuery == ""
	}
	data, err := io.ReadAll(io.LimitReader(request.Body, 1))
	return err == nil && len(data) == 0 && request.URL.RawQuery == ""
}
