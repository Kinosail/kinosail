package main

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestR16HistoryProjectionEmptyOmissionContract(t *testing.T) {
	for _, raw := range []json.RawMessage{nil, []byte(`[]`)} {
		rows, err := r16DecodeHistory(raw, true)
		if err != nil || len(rows) != 0 {
			t.Fatal("published empty History representation was rejected")
		}
	}
	for _, raw := range []json.RawMessage{nil, []byte(`null`), []byte(`{}`), []byte(`true`)} {
		if _, err := r16DecodeHistory(raw, false); err == nil {
			t.Fatal("required History array was not enforced")
		}
	}
	for _, raw := range []json.RawMessage{[]byte(`null`), []byte(`{}`), []byte(`true`)} {
		if _, err := r16DecodeHistory(raw, true); err == nil {
			t.Fatal("present invalid History value was accepted as empty")
		}
	}
}

func r16DecodeHistory(raw json.RawMessage, allowEmpty bool) ([]r16History, error) {
	if len(raw) == 0 && allowEmpty {
		return nil, nil
	}
	var rows []r16History
	if json.Unmarshal(raw, &rows) != nil || rows == nil {
		return nil, errors.New("actual R16 History array invalid")
	}
	return rows, nil
}
