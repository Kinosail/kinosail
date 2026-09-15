package httpguard

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// DecodeUniqueJSON accepts one bounded object with no unknown or repeated fields.
// Case-folding also rejects duplicate aliases accepted by encoding/json's struct decoder.
func DecodeUniqueJSON(reader io.Reader, maximum int64, target any) error {
	if reader == nil || maximum < 1 {
		return errors.New("invalid JSON object")
	}
	data, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil || int64(len(data)) > maximum || !utf8.Valid(data) {
		return errors.New("invalid JSON object")
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' {
		return errors.New("invalid JSON object")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if !uniqueValue(d, 0) {
		return errors.New("invalid JSON object")
	}
	if _, err = d.Token(); err != io.EOF {
		return errors.New("invalid JSON object")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(target) != nil {
		return errors.New("invalid JSON object")
	}
	return nil
}

func uniqueValue(d *json.Decoder, depth int) bool {
	if depth > 32 {
		return false
	}
	token, err := d.Token()
	if err != nil {
		return false
	}
	switch token {
	case json.Delim('{'):
		keys := map[string]bool{}
		for d.More() {
			token, err = d.Token()
			key, ok := token.(string)
			if err != nil || !ok || keys[strings.ToLower(key)] {
				return false
			}
			keys[strings.ToLower(key)] = true
			if !uniqueValue(d, depth+1) {
				return false
			}
		}
		token, err = d.Token()
		return err == nil && token == json.Delim('}')
	case json.Delim('['):
		for d.More() {
			if !uniqueValue(d, depth+1) {
				return false
			}
		}
		token, err = d.Token()
		return err == nil && token == json.Delim(']')
	default:
		_, delimiter := token.(json.Delim)
		return !delimiter
	}
}
