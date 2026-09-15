package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

func decodeSubtitleValue(raw json.RawMessage, target any) bool {
	return !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) && json.Unmarshal(raw, target) == nil
}

func readSubtitleJSON(writer http.ResponseWriter, request *http.Request, target any) bool { //nolint:cyclop // One strict boundary rejects malformed, unknown, duplicate, trailing, and oversized input.
	request.Body = http.MaxBytesReader(writer, request.Body, 4096)
	data, err := io.ReadAll(request.Body)
	if err != nil || len(data) == 0 {
		apiError(writer, errors.New("invalid JSON request"), http.StatusBadRequest)
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if delimiter, ok := token.(json.Delim); err != nil || !ok || delimiter != '{' {
		apiError(writer, errors.New("invalid JSON request"), http.StatusBadRequest)
		return false
	}
	seen := make(map[string]bool)
	for decoder.More() {
		field, err := decoder.Token()
		name, ok := field.(string)
		canonical := strings.ToLower(name)
		if err != nil || !ok || seen[canonical] {
			apiError(writer, errors.New("invalid JSON request"), http.StatusBadRequest)
			return false
		}
		seen[canonical] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			apiError(writer, errors.New("invalid JSON request"), http.StatusBadRequest)
			return false
		}
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') || decoder.Decode(&struct{}{}) != io.EOF {
		apiError(writer, errors.New("invalid JSON request"), http.StatusBadRequest)
		return false
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		apiError(writer, errors.New("invalid JSON request"), http.StatusBadRequest)
		return false
	}
	return true
}
