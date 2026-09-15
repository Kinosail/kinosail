package supporter

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"slices"
)

func jsonObject(data []byte) (map[string]json.RawMessage, error) { //nolint:cyclop // Strict duplicate-aware JSON parsing is one boundary.
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if delimiter, ok := token.(json.Delim); err != nil || !ok || delimiter != '{' {
		return nil, errors.New("supporter value must be an object")
	}
	object := make(map[string]json.RawMessage)
	for decoder.More() {
		key, keyErr := decoder.Token()
		field, ok := key.(string)
		if keyErr != nil || !ok {
			return nil, errors.New("supporter field is invalid")
		}
		if _, exists := object[field]; exists {
			return nil, errors.New("supporter value contains duplicate fields")
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, errors.New("supporter field value is invalid")
		}
		object[field] = value
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, errors.New("supporter value is invalid")
	}
	return object, nil
}

func exactFields(object map[string]json.RawMessage, required, optional []string) bool {
	allowed := append(slices.Clone(required), optional...)
	for _, field := range required {
		if _, found := object[field]; !found {
			return false
		}
	}
	for field := range object {
		if !slices.Contains(allowed, field) {
			return false
		}
	}
	return true
}

func jsonString(raw json.RawMessage) (string, bool) {
	if len(raw) < 2 || raw[0] != '"' {
		return "", false
	}
	var value string
	err := json.Unmarshal(raw, &value)
	return value, err == nil
}

func jsonBool(raw json.RawMessage) (bool, bool) {
	if string(raw) == "true" {
		return true, true
	}
	if string(raw) == "false" {
		return false, true
	}
	return false, false
}

func validJSONUnicode(record []byte) bool {
	inString := false
	for index := 0; index < len(record); index++ {
		switch record[index] {
		case '"':
			inString = !inString
		case '\\':
			next, ok := validUnicodeEscape(record, index, inString)
			if !ok {
				return false
			}
			index = next
		}
	}
	return true
}

func validUnicodeEscape(record []byte, start int, inString bool) (int, bool) { //nolint:cyclop // Unicode escape validation remains below the repository complexity ceiling.
	if !inString || start+1 >= len(record) {
		return start, true
	}
	if record[start+1] != 'u' {
		return start + 1, true
	}
	value, ok := unicodeEscape(record, start)
	if !ok || value >= 0xdc00 && value <= 0xdfff {
		return start, false
	}
	if value < 0xd800 || value > 0xdbff {
		return start + 5, true
	}
	low, lowOK := unicodeEscape(record, start+6)
	return start + 11, lowOK && low >= 0xdc00 && low <= 0xdfff
}

func unicodeEscape(record []byte, start int) (uint16, bool) {
	if start+6 > len(record) || record[start] != '\\' || record[start+1] != 'u' {
		return 0, false
	}
	var decoded [2]byte
	count, err := hex.Decode(decoded[:], record[start+2:start+6])
	return uint16(decoded[0])<<8 | uint16(decoded[1]), err == nil && count == 2
}
