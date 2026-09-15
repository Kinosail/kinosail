package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"reflect"
	"strings"
)

func readJSON(writer http.ResponseWriter, request *http.Request, target any, limit int64) bool {
	data, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, limit))
	if err != nil || len(data) == 0 || duplicateJSONKey(data) || !validJSONShape(data, target) {
		apiError(writer, errors.New("invalid JSON request"), http.StatusBadRequest)
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		apiError(writer, errors.New("invalid JSON request"), http.StatusBadRequest)
		return false
	}
	// duplicateJSONKey already rejects trailing values before decoding.
	return true
}

func validJSONShape(data []byte, target any) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return false
	}
	return matchesJSONShape(value, reflect.TypeOf(target))
}

func matchesJSONShape(value any, target reflect.Type) bool {
	if value == nil || target == nil {
		return false
	}
	for target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	switch {
	case target.Kind() == reflect.Struct:
		return matchesJSONObject(value, target)
	case target.Kind() == reflect.Slice || target.Kind() == reflect.Array:
		return matchesJSONArray(value, target.Elem())
	case target.Kind() == reflect.Map:
		return matchesJSONMap(value, target.Elem())
	}
	return true
}

func matchesJSONObject(value any, target reflect.Type) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return true
	}
	fields := jsonFields(target)
	for name, fieldValue := range object {
		fieldType, found := fields[name]
		if !found || !matchesJSONShape(fieldValue, fieldType) {
			return false
		}
	}
	return true
}

func matchesJSONArray(value any, element reflect.Type) bool {
	items, ok := value.([]any)
	if !ok {
		return true
	}
	for _, item := range items {
		if !matchesJSONShape(item, element) {
			return false
		}
	}
	return true
}

func matchesJSONMap(value any, element reflect.Type) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return true
	}
	for _, item := range object {
		if !matchesJSONShape(item, element) {
			return false
		}
	}
	return true
}

func jsonFields(target reflect.Type) map[string]reflect.Type {
	for target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	fields := make(map[string]reflect.Type, target.NumField())
	for index := range target.NumField() {
		field := target.Field(index)
		if !field.IsExported() {
			continue
		}
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if field.Anonymous && name == "" {
			maps.Copy(fields, jsonFields(field.Type))
			continue
		}
		if name == "" {
			name = field.Name
		}
		fields[name] = field.Type
	}
	return fields
}

func duplicateJSONKey(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if checkJSONValue(decoder) != nil {
		return true
	}
	_, err := decoder.Token()
	return !errors.Is(err, io.EOF)
}

func checkJSONValue(decoder *json.Decoder) error { //nolint:cyclop // Recursive token validation detects duplicate keys at every object depth.
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delimiter {
	case '{':
		return checkJSONObject(decoder)
	case '[':
		return checkJSONArray(decoder)
	default:
		return errors.New("invalid JSON delimiter")
	}
}

func checkJSONObject(decoder *json.Decoder) error {
	seen := map[string]bool{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		key, ok := keyToken.(string)
		key = strings.ToLower(key)
		if err != nil || !ok || seen[key] {
			return errors.New("duplicate or invalid JSON key")
		}
		seen[key] = true
		if err := checkJSONValue(decoder); err != nil {
			return err
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return errors.New("invalid JSON object")
	}
	return nil
}

func checkJSONArray(decoder *json.Decoder) error {
	for decoder.More() {
		if err := checkJSONValue(decoder); err != nil {
			return err
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim(']') {
		return errors.New("invalid JSON array")
	}
	return nil
}

func writeJSON(writer http.ResponseWriter, value any, status int) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func apiError(writer http.ResponseWriter, err error, status int) {
	writeJSON(writer, map[string]string{"error": err.Error()}, status)
}
