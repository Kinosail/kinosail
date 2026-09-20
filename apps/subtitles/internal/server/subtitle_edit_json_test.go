package server

import (
	"strings"
	"testing"
)

func TestSubtitleEditJSONBounds(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		valid        bool
	}{
		{"maximum depth", `{"a":[[[1]]]}`, true},
		{"excess depth", `{"a":[[[[1]]]]}`, false},
		{"maximum values", `{"a":[` + strings.Repeat("1,", 125) + `1]}`, true},
		{"excess values", `{"a":[` + strings.Repeat("1,", 126) + `1]}`, false},
		{"case duplicate", `{"a":{"Key":1,"key":2}}`, false},
		{"null nested", `{"a":[null]}`, false},
		{"root array", `[]`, false},
		{"root scalar", `1`, false},
		{"trailing document", `{} {}`, false},
		{"empty object", `{}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validSubtitleEditJSON([]byte(tc.source)); got != tc.valid {
				t.Fatalf("valid=%v, want %v", got, tc.valid)
			}
		})
	}
}
