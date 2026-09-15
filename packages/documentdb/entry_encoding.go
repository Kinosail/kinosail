package documentdb

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

func decodeEntries(data []byte) (map[string]json.RawMessage, error) {
	if len(data) > DocumentSizeLimit || !utf8.Valid(data) {
		return nil, errors.New("invalid record encoding")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := uniqueJSONValue(decoder, 0); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("invalid record document")
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil || len(entries) > 1_000_000 {
		return nil, errors.New("record document must be a bounded object")
	}
	return entries, nil
}

func uniqueJSONValue(decoder *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("record nesting is too deep")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delimiter != '{' && delimiter != '[' {
		return errors.New("invalid record document")
	}
	keys := make(map[string]struct{})
	count := 0
	for decoder.More() {
		count++
		if count > 1_000_000 {
			return errors.New("record cardinality is too large")
		}
		if err := uniqueJSONKey(decoder, keys, delimiter == '{'); err != nil {
			return err
		}
		if err := uniqueJSONValue(decoder, depth+1); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}

func entriesBudget(entries map[string]json.RawMessage) (int, error) {
	total := 1
	for key, value := range entries {
		if key == "" || len(key) > 256 || !utf8.ValidString(key) {
			return 0, errors.New("invalid record key")
		}
		total += entryCost(key, value)
		if total > DocumentSizeLimit {
			return 0, errors.New("record document is too large")
		}
	}
	return total, nil
}

func uniqueJSONKey(decoder *json.Decoder, keys map[string]struct{}, object bool) error {
	if !object {
		return nil
	}
	keyToken, err := decoder.Token()
	if err != nil {
		return err
	}
	key, ok := keyToken.(string)
	if !ok {
		return errors.New("invalid record key")
	}
	if _, exists := keys[key]; exists {
		return errors.New("duplicate record key")
	}
	keys[key] = struct{}{}
	return nil
}
