package httpguard

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLogValuesAreBoundedAndCannotForgeRecords(t *testing.T) {
	t.Parallel()
	for method, want := range map[string]string{"GET": "GET", "HEAD": "HEAD", "POST": "POST", "PUT": "PUT", "PATCH": "PATCH", "DELETE": "DELETE", "OPTIONS": "OPTIONS", "get": "OTHER", "PROPFIND": "OTHER", "GET\r\nX": "OTHER", strings.Repeat("A", 4096): "OTHER"} {
		if got := LogMethod(method); got != want {
			t.Errorf("LogMethod(%q) = %q, want %q", method, got, want)
		}
	}
	for value, want := range map[string]string{
		"bytes=0-99":                         "bytes=0-99",
		"line\r\n{\"msg\":\"forged\"}":       "line{\"msg\":\"forged\"}",
		"tab\tbell\aescape\x1b[2Jnul\x00end": "tabbellescape[2Jnulend",
		"next\u2028line\u0085":               "nextline",
		"invalid\xffutf8":                    "invalidutf8",
	} {
		if got := LogText(value, 64); got != want {
			t.Errorf("LogText(%q) = %q, want %q", value, got, want)
		}
	}
	if LogText("value", 0) != "" || LogText(strings.Repeat("x", 1<<20), 8) != "xxxxxxxx" {
		t.Fatal("zero or large bounds were not applied")
	}
	if got := LogText(strings.Repeat("é", 40), 9); got != "éééé" || !utf8.ValidString(got) {
		t.Fatalf("bounded multibyte value = %q", got)
	}
}
