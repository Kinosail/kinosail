package server_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type subtitleActionReview struct {
	Fingerprint string
	Restorable  bool
	Current     struct{ Cues []subtitlePairingCue }
}

const subtitleActionInitial = "1\n00:00:01,000 --> 00:00:02,000\nFirst dialogue\n\n2\n00:00:04,000 --> 00:00:05,000\nLater dialogue\n"

func TestWriteUIStateFixturesSubtitleActionSaveControl(t *testing.T) {
	t.Parallel()
	handler, base, target := subtitleInspectorFixture(t, subtitleActionInitial)
	before := subtitleActionRead(t, handler, base)
	input, _ := json.Marshal(map[string]any{"language": "en", "fingerprint": before.Fingerprint, "offsetMilliseconds": 500})
	preview := requestJSON(t, handler, http.MethodPost, base+"/preview", string(input))
	if preview.Code != http.StatusOK {
		t.Fatalf("preview control = %d %s", preview.Code, preview.Body.String())
	}
	files := subtitleActionBrowserFiles(t, handler, base, "save", preview.Body.Bytes())
	saved := requestJSON(t, handler, http.MethodPost, base+"/apply", string(input))
	if saved.Code != http.StatusOK {
		t.Fatalf("save control = %d %s", saved.Code, saved.Body.String())
	}
	current := subtitleActionRead(t, handler, base)
	assertSubtitleActionCues(t, current, []subtitlePairingCue{{1.5, 2.5, "First dialogue"}, {4.5, 5.5, "Later dialogue"}})
	assertSubtitleActionFingerprint(t, current, target)
	assertSubtitleActionBytes(t, target+".kinosail.bak", []byte(subtitleActionInitial))
	history := subtitleActionHistory(t, handler, []string{"updated"}, []string{"manual"})
	stored, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	duplicate := requestJSON(t, handler, http.MethodPost, base+"/apply", string(input))
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("stale repeated save = %d %s", duplicate.Code, duplicate.Body.String())
	}
	assertSubtitleActionBytes(t, target, stored)
	assertSubtitleActionBytes(t, target+".kinosail.bak", []byte(subtitleActionInitial))
	_ = subtitleActionHistory(t, handler, []string{"updated"}, []string{"manual"})
	files["after.json"] = requestApp(t, handler, http.MethodGet, base+"/inspect?language=en", "").Body.Bytes()
	files["history.json"] = history
	writeSubtitleActionFiles(t, "save", files)
}

func TestWriteUIStateFixturesSubtitleActionRestoreControl(t *testing.T) {
	t.Parallel()
	handler, base, target := subtitleInspectorFixture(t, subtitleActionInitial)
	before := subtitleActionRead(t, handler, base)
	input, _ := json.Marshal(map[string]any{"language": "en", "fingerprint": before.Fingerprint, "offsetMilliseconds": 500})
	saved := requestJSON(t, handler, http.MethodPost, base+"/apply", string(input))
	if saved.Code != http.StatusOK {
		t.Fatalf("restore setup save = %d %s", saved.Code, saved.Body.String())
	}
	previous, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	files := subtitleActionBrowserFiles(t, handler, base, "restore", nil)
	response := requestJSON(t, handler, http.MethodPost, base+"/restore", `{"language":"en"}`)
	if response.Code != http.StatusNoContent {
		t.Fatalf("restore control = %d %s", response.Code, response.Body.String())
	}
	current := subtitleActionRead(t, handler, base)
	assertSubtitleActionCues(t, current, []subtitlePairingCue{{1, 2, "First dialogue"}, {4, 5, "Later dialogue"}})
	assertSubtitleActionFingerprint(t, current, target)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	assertSubtitleActionBytes(t, target+".kinosail.bak", previous)
	files["after.json"] = requestApp(t, handler, http.MethodGet, base+"/inspect?language=en", "").Body.Bytes()
	files["history.json"] = subtitleActionHistory(t, handler, []string{"restored", "updated"}, []string{"restore", "manual"})
	writeSubtitleActionFiles(t, "restore", files)
}

func subtitleActionRead(t *testing.T, handler http.Handler, base string) subtitleActionReview {
	t.Helper()
	response := requestApp(t, handler, http.MethodGet, base+"/inspect?language=en", "")
	var review subtitleActionReview
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &review) != nil || len(review.Fingerprint) != 64 {
		t.Fatalf("action inspection = %d %s", response.Code, response.Body.String())
	}
	return review
}

func assertSubtitleActionCues(t *testing.T, review subtitleActionReview, expected []subtitlePairingCue) {
	t.Helper()
	if !review.Restorable || !reflect.DeepEqual(review.Current.Cues, expected) {
		t.Fatalf("action cues/recovery = %+v, want %v with recovery", review, expected)
	}
}

func assertSubtitleActionFingerprint(t *testing.T, review subtitleActionReview, target string) {
	t.Helper()
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if review.Fingerprint != hex.EncodeToString(digest[:]) {
		t.Fatal("public fingerprint does not identify the installed bytes")
	}
}

func assertSubtitleActionBytes(t *testing.T, path string, expected []byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != string(expected) {
		t.Fatalf("subtitle action data integrity mismatch: %v", err)
	}
}

func subtitleActionHistory(t *testing.T, handler http.Handler, actions, reasons []string) []byte {
	t.Helper()
	response := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-library?view=history", "")
	var history struct {
		Matched int
		History []struct{ Action, Reason string }
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &history) != nil || history.Matched != len(actions) || len(history.History) != len(actions) {
		t.Fatalf("action history = %d %s", response.Code, response.Body.String())
	}
	for index, item := range history.History {
		if item.Action != actions[index] || item.Reason != reasons[index] {
			t.Fatalf("history %d = %+v, want %s/%s", index, item, actions[index], reasons[index])
		}
	}
	return append([]byte(nil), response.Body.Bytes()...)
}

func subtitleActionBrowserFiles(t *testing.T, handler http.Handler, base, action string, preview []byte) map[string][]byte {
	t.Helper()
	files := map[string][]byte{"preview.json": append([]byte(nil), preview...)}
	id := strings.TrimPrefix(base, "/api/v1/subtitle-library/")
	paths := map[string]string{"inspector.html": "/subtitles/inspect/" + id, "before.json": base + "/inspect?language=en", "before-history.json": "/api/v1/subtitle-library?view=history", "dashboard.html": "/?view=library", "history.html": "/?view=history", "app.css": "/static/app.css", "subtitle-inspector.css": "/static/subtitle-inspector.css", "subtitle-inspector.js": "/static/subtitle-inspector.js", "subtitle-status.js": "/static/subtitle-status.js", "theme.js": "/static/theme.js", "icon.svg": "/static/icon.svg", "manrope.woff2": "/static/manrope.woff2"}
	for name, path := range paths {
		response := requestApp(t, handler, http.MethodGet, path, "")
		if response.Code != http.StatusOK {
			t.Fatalf("%s action fixture %s = %d", action, name, response.Code)
		}
		files[name] = append([]byte(nil), response.Body.Bytes()...)
	}
	return files
}

func writeSubtitleActionFiles(t *testing.T, action string, files map[string][]byte) {
	t.Helper()
	root := os.Getenv("KINOSAIL_SUBTITLE_ACTION_FIXTURE_DIR")
	if root == "" {
		return
	}
	dir := filepath.Join(root, action)
	if err := os.MkdirAll(dir, 0o700); err != nil { //nolint:gosec // The caller chooses its isolated artifact directory.
		t.Fatal(err)
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil { //nolint:gosec // Fixed artifact filenames under the caller's chosen directory.
			t.Fatal(err)
		}
	}
}
