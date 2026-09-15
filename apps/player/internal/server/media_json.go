package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

func uniqueMediaJSON(decoder *json.Decoder, depth int) bool {
	if depth > 16 {
		return false
	}
	token, err := decoder.Token()
	if err != nil {
		return false
	}
	delim, composite := token.(json.Delim)
	if !composite {
		return true
	}
	if delim != '{' && delim != '[' {
		return false
	}
	return uniqueMediaComposite(decoder, depth, delim)
}

func uniqueMediaComposite(decoder *json.Decoder, depth int, delim json.Delim) bool {
	seen := map[string]bool{}
	for decoder.More() {
		if delim == '{' && !readUniqueMediaKey(decoder, seen) {
			return false
		}
		if !uniqueMediaJSON(decoder, depth+1) {
			return false
		}
	}
	end, err := decoder.Token()
	if err != nil {
		return false
	}
	if delim == '{' {
		return end == json.Delim('}')
	}
	return end == json.Delim(']')
}

func readUniqueMediaKey(decoder *json.Decoder, seen map[string]bool) bool {
	key, err := decoder.Token()
	if err != nil {
		return false
	}
	name, ok := key.(string)
	if !ok || seen[name] {
		return false
	}
	seen[name] = true
	return true
}

func readMediaJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	var raw json.RawMessage
	if !readJSON(writer, request, &raw) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if !uniqueMediaJSON(decoder, 0) {
		apiError(writer, errMediaPreferences, http.StatusBadRequest)
		return false
	}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		apiError(writer, errMediaPreferences, http.StatusBadRequest)
		return false
	}
	return true
}
