package server_test

import (
	"encoding/base64"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestSubtitleInspectorTTMLMillisecondImportRemainsReadable(t *testing.T) {
	original := `<tt><body><div><p begin="2.001s" end="2.002s">Brief cue</p></div></body></tt>`
	handler, base, target := subtitleInspectorFixture(t, "")
	input := `{"language":"en","fingerprint":"missing","data":"` + base64.StdEncoding.EncodeToString([]byte(original)) + `"}`
	preview := requestJSON(t, handler, http.MethodPost, base+"/preview", input)
	if preview.Code != http.StatusOK {
		t.Fatalf("preview = %d %s", preview.Code, preview.Body.String())
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("preview changed the sidecar")
	}
	saved := requestJSON(t, handler, http.MethodPost, base+"/apply", input)
	if saved.Code != http.StatusOK {
		t.Fatalf("save = %d %s", saved.Code, saved.Body.String())
	}
	data, err := os.ReadFile(target)
	if err != nil || !strings.Contains(string(data), "00:00:02,001 --> 00:00:02,002") {
		t.Fatalf("saved millisecond timing = %q, %v", data, err)
	}
	inspection := requestApp(t, handler, http.MethodGet, base+"/inspect?language=en", "")
	if inspection.Code != http.StatusOK || !strings.Contains(inspection.Body.String(), "Brief cue") {
		t.Fatalf("reinspection = %d %s", inspection.Code, inspection.Body.String())
	}
	export := requestApp(t, handler, http.MethodGet, base+"/export?language=en&format=original", "")
	if export.Code != http.StatusOK || export.Body.String() != original {
		t.Fatalf("original export = %d %s", export.Code, export.Body.String())
	}
}

func TestSubtitleInspectorRejectsUnrepresentableTTMLBeforeWriting(t *testing.T) {
	for _, timing := range []string{
		`begin="2.0011s" end="2.0019s"`,
		`begin="2001.1ms" end="2001.9ms"`,
		`begin="2.0011s" dur="0.0008s"`,
	} {
		t.Run(timing, func(t *testing.T) {
			handler, base, target := subtitleInspectorFixture(t, "")
			original := `<tt><body><div><p ` + timing + `>Brief cue</p></div></body></tt>`
			input := `{"language":"en","fingerprint":"missing","data":"` + base64.StdEncoding.EncodeToString([]byte(original)) + `"}`
			for _, operation := range []string{"preview", "apply"} {
				response := requestJSON(t, handler, http.MethodPost, base+"/"+operation, input)
				if response.Code != http.StatusBadRequest {
					t.Errorf("%s accepted unrepresentable timing: %d", operation, response.Code)
				}
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					t.Errorf("%s changed the sidecar: %v", operation, err)
				}
			}
		})
	}
}
