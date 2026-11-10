package server_test

import (
	"encoding/base64"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestSubtitleInspectorTTMLImportKeepsWrappedSourceTextOnOneLine(t *testing.T) {
	original := "<tt xmlns=\"http://www.w3.org/ns/ttml\">\n  <body>\n    <div>\n      <p begin=\"1s\" end=\"3s\">\n        Hello there,\n        <span>General</span> Kenobi<br/>\n        Second line\n      </p>\n      <p begin=\"4s\" end=\"5s\" xml:space=\"preserve\">Kept\nbreak</p>\n    </div>\n  </body>\n</tt>\n"
	handler, base, target := subtitleInspectorFixture(t, "")
	input := `{"language":"en","fingerprint":"missing","data":"` + base64.StdEncoding.EncodeToString([]byte(original)) + `"}`
	saved := requestJSON(t, handler, http.MethodPost, base+"/apply", input)
	if saved.Code != http.StatusOK {
		t.Fatalf("save = %d %s", saved.Code, saved.Body.String())
	}
	data, err := os.ReadFile(target)
	want := "1\n00:00:01,000 --> 00:00:03,000\nHello there, General Kenobi\nSecond line\n\n2\n00:00:04,000 --> 00:00:05,000\nKept\nbreak\n\n"
	if err != nil || strings.ReplaceAll(string(data), "\r\n", "\n") != want {
		t.Fatalf("saved subtitle = %q, %v; want %q", data, err, want)
	}
}
