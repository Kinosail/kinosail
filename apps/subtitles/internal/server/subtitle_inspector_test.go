package server_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func subtitleInspectorFixture(t *testing.T, subtitle string) (http.Handler, string, string) {
	t.Helper()
	media := t.TempDir()
	writeTestFile(t, filepath.Join(media, "Arrival.mp4"), "video")
	target := filepath.Join(media, "Arrival.en.srt")
	if subtitle != "" {
		writeTestFile(t, target, subtitle)
	}
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	response := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-library?view=library", "")
	var inventory struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if json.Unmarshal(response.Body.Bytes(), &inventory) != nil || len(inventory.Items) != 1 {
		t.Fatalf("library = %s", response.Body.String())
	}
	return handler, "/api/v1/subtitle-library/" + inventory.Items[0].ID, target
}

func TestSubtitleInspectorImportsWithPreviewOriginalAndRecovery(t *testing.T) {
	original := "[Script Info]\n[Events]\nFormat: Start, End, Text\nDialogue: 0:00:01.00,0:00:03.00,Hello world\n"
	handler, base, target := subtitleInspectorFixture(t, "")
	input := `{"language":"en","fingerprint":"missing","data":"` + base64.StdEncoding.EncodeToString([]byte(original)) + `"}`
	preview := requestJSON(t, handler, http.MethodPost, base+"/preview", input)
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), "Hello world") {
		t.Fatalf("preview = %d %s", preview.Code, preview.Body.String())
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("preview wrote a sidecar")
	}
	saved := requestJSON(t, handler, http.MethodPost, base+"/apply", input)
	if saved.Code != http.StatusOK || !strings.Contains(saved.Body.String(), `"originalAvailable":true`) {
		t.Fatalf("save = %d %s", saved.Code, saved.Body.String())
	}
	export := requestApp(t, handler, http.MethodGet, base+"/export?language=en&format=original", "")
	if export.Code != http.StatusOK || export.Body.String() != original {
		t.Fatalf("original = %d %s", export.Code, export.Body.String())
	}
	var review struct {
		Fingerprint string `json:"fingerprint"`
	}
	if json.Unmarshal(saved.Body.Bytes(), &review) != nil {
		t.Fatal("invalid saved review")
	}
	shift := `{"language":"en","fingerprint":"` + review.Fingerprint + `","offsetMilliseconds":750}`
	changed := requestJSON(t, handler, http.MethodPost, base+"/apply", shift)
	data, _ := os.ReadFile(target)
	if changed.Code != http.StatusOK || !strings.Contains(string(data), "00:00:01,750") {
		t.Fatalf("shift = %d %s, %s", changed.Code, changed.Body.String(), data)
	}
	export = requestApp(t, handler, http.MethodGet, base+"/export?language=en&format=original", "")
	if export.Body.String() != original {
		t.Fatal("editing lost the provider-format original")
	}
	restored := requestJSON(t, handler, http.MethodPost, base+"/restore", `{"language":"en"}`)
	data, _ = os.ReadFile(target)
	if restored.Code != http.StatusNoContent || !strings.Contains(string(data), "00:00:01,000") {
		t.Fatalf("restore = %d %s", restored.Code, data)
	}
}

func TestSubtitleInspectorRejectsStaleEditsWithoutWriting(t *testing.T) {
	initial := "1\n00:00:01,000 --> 00:00:03,000\nOriginal\n"
	handler, base, target := subtitleInspectorFixture(t, initial)
	response := requestApp(t, handler, http.MethodGet, base+"/inspect?language=en", "")
	var review struct {
		Fingerprint string `json:"fingerprint"`
	}
	if json.Unmarshal(response.Body.Bytes(), &review) != nil {
		t.Fatal("invalid inspection")
	}
	changed := strings.Replace(initial, "Original", "Someone else's edit", 1)
	writeTestFile(t, target, changed)
	result := requestJSON(t, handler, http.MethodPost, base+"/apply", `{"language":"en","fingerprint":"`+review.Fingerprint+`","offsetMilliseconds":1000}`)
	data, _ := os.ReadFile(target)
	if result.Code != http.StatusConflict || string(data) != changed {
		t.Fatalf("stale edit = %d %s", result.Code, data)
	}
	if _, err := os.Stat(target + ".kinosail.bak"); !os.IsNotExist(err) {
		t.Fatal("stale edit wrote a backup")
	}
}

func TestSubtitleInspectorRejectsMalformedEditsWithoutSideEffects(t *testing.T) {
	initial := "1\n00:00:01,000 --> 00:00:03,000\nOriginal\n"
	handler, base, target := subtitleInspectorFixture(t, initial)
	for _, input := range []string{
		`{}`, `{"language":null}`, `{"language":""}`, `{"language":"../en"}`, `{"language":"en","unknown":true}`,
		`{"language":"en","Language":"es"}`, `{"language":"en"} {}`, `{"language":"en","encoding":"guess"}`,
		`{"language":"en","offsetMilliseconds":120001}`, `{"language":"en","offsetMilliseconds":-120001}`,
		`{"language":"en","automaticSync":true,"offsetMilliseconds":1}`, `{"language":"en","data":"not-base64"}`,
		`{"language":"en","anchors":[{}]}`, `{"language":"en","anchors":[{"atMilliseconds":1,"offsetMilliseconds":0,"offsetMilliseconds":1}]}`,
		`{"language":"en","anchors":[{"atMilliseconds":20,"offsetMilliseconds":0},{"atMilliseconds":10,"offsetMilliseconds":0}]}`,
		`{"language":"en","anchors":[{"atMilliseconds":0,"offsetMilliseconds":120000},{"atMilliseconds":1,"offsetMilliseconds":0}]}`,
		`{"language":"en","data":"` + base64.StdEncoding.EncodeToString(make([]byte, (4<<20)+1)) + `"}`,
	} {
		result := requestJSON(t, handler, http.MethodPost, base+"/preview", input)
		if result.Code != http.StatusBadRequest {
			t.Errorf("invalid edit status = %d for %.100s", result.Code, input)
		}
		data, _ := os.ReadFile(target)
		if string(data) != initial {
			t.Fatal("invalid edit changed the sidecar")
		}
		if _, err := os.Stat(target + ".kinosail.bak"); !os.IsNotExist(err) {
			t.Fatal("invalid edit wrote a backup")
		}
	}
	result := requestJSON(t, handler, http.MethodPost, base+"/apply", `{"language":"en"}`)
	if result.Code != http.StatusBadRequest {
		t.Fatal("save accepted a missing fingerprint")
	}
}

func TestSubtitleInspectorPreservesSameFormatWebVTTExport(t *testing.T) {
	handler, base, target := subtitleInspectorFixture(t, "")
	original := "WEBVTT\n\n00:01.000 --> 00:03.000 line:10% position:20%\n<v Alex>Hello</v>\n"
	writeTestFile(t, strings.TrimSuffix(target, ".srt")+".vtt", original)
	requestApp(t, handler, http.MethodPost, "/scan", "")
	result := requestApp(t, handler, http.MethodGet, base+"/export?language=en&format=vtt", "")
	if result.Code != http.StatusOK || result.Body.String() != original {
		t.Fatalf("WebVTT changed = %d %q", result.Code, result.Body.String())
	}
}

func TestSubtitleInspectorCorrectsTextBeforeSavingAndRetainsOriginal(t *testing.T) {
	initial := "1\n00:00:01,000 --> 00:00:03,000\nHelo world\n"
	handler, base, target := subtitleInspectorFixture(t, initial)
	response := requestApp(t, handler, http.MethodGet, base+"/inspect?language=en", "")
	var review struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &review); err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(map[string]any{"language": "en", "fingerprint": review.Fingerprint, "text": strings.Replace(initial, "Helo", "Hello", 1)})
	preview := requestJSON(t, handler, http.MethodPost, base+"/preview", string(input))
	before, _ := os.ReadFile(target)
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), "Hello world") || string(before) != initial {
		t.Fatalf("preview = %d %s", preview.Code, preview.Body.String())
	}
	saved := requestJSON(t, handler, http.MethodPost, base+"/apply", string(input))
	original := requestApp(t, handler, http.MethodGet, base+"/export?language=en&format=original", "")
	current, _ := os.ReadFile(target)
	if saved.Code != http.StatusOK || !strings.Contains(string(current), "Hello world") || original.Body.String() != initial {
		t.Fatalf("saved=%d original=%q current=%q", saved.Code, original.Body.String(), current)
	}
}

func TestSubtitleDraftHTTPRejectsMalformedInputWithoutSideEffects(t *testing.T) {
	initial := "1\n00:00:01,000 --> 00:00:03,000\nOriginal\n"
	handler, base, target := subtitleInspectorFixture(t, initial)
	for _, input := range []string{`{}`, `{"language":"en","language":"es","method":"ocr","action":"start"}`, `{"language":"en","method":"ocr","action":"start","unexpected":1}`, `{"language":null,"method":"ocr","action":"start"}`, `{"language":"en","method":"remote","action":"start"}`, `{"language":"en","method":"ocr","action":"cancel"}`, strings.Repeat(" ", 1025)} {
		result := requestJSON(t, handler, http.MethodPost, base+"/draft", input)
		if result.Code != http.StatusBadRequest {
			t.Errorf("invalid draft = %d %s", result.Code, result.Body.String())
		}
	}
	result := requestApp(t, handler, http.MethodGet, base+"/draft?language=en", "")
	after, _ := os.ReadFile(target)
	if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), `"state":"none"`) || string(after) != initial {
		t.Fatal("invalid draft request changed state")
	}
}
