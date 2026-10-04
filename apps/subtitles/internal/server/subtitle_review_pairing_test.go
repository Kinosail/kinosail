package server_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type subtitlePairingCue struct {
	Start, End float64
	Text       string
}

type subtitlePairingComparison struct {
	Current, Proposed []int
	Kind              string
}

type subtitlePairingAPIReview struct {
	Current, Proposed struct{ Cues []subtitlePairingCue }
	Comparison        []subtitlePairingComparison
}

func TestWriteUIStateFixturesSubtitlePairingPreservesDialogueCorrespondenceThroughCleanup(t *testing.T) {
	t.Parallel()
	initial := "1\n00:00:01,000 --> 00:00:02,000\nDownloaded from www.example.com\n\n2\n00:00:03,000 --> 00:00:04,000\nHello there\n\n3\n00:00:04,000 --> 00:00:05,000\nHello there\n\n4\n00:00:07,000 --> 00:00:08,000\nA later line\n"
	handler, base, target := subtitleInspectorFixture(t, initial)
	response := requestJSON(t, handler, http.MethodPost, base+"/preview", `{"language":"en","removeCredits":true,"mergeRepeated":true,"offsetMilliseconds":750}`)
	cleanupJSON := append([]byte(nil), response.Body.Bytes()...)
	var review subtitlePairingAPIReview
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &review) != nil {
		t.Fatalf("cleanup preview = %d %s", response.Code, response.Body.String())
	}
	imported := "1\n00:00:03,000 --> 00:00:05,000\nBonjour <i>ami</i>\n"
	input, _ := json.Marshal(map[string]any{"language": "en", "data": base64.StdEncoding.EncodeToString([]byte(imported)), "removeCredits": true, "mergeRepeated": true})
	response = requestJSON(t, handler, http.MethodPost, base+"/preview", string(input))
	var importedReview subtitlePairingAPIReview
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &importedReview) != nil {
		t.Fatalf("import comparison = %d %s", response.Code, response.Body.String())
	}
	writeSubtitlePairingBrowserFixture(t, handler, base, cleanupJSON, response.Body.Bytes())
	response = requestJSON(t, handler, http.MethodPost, base+"/preview", `{"language":"en","text":"1\n00:00:03,000 --> 00:00:05,000\nIndependently edited dialogue\n"}`)
	var editedReview subtitlePairingAPIReview
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &editedReview) != nil {
		t.Fatalf("edited comparison = %d %s", response.Code, response.Body.String())
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != initial {
		t.Fatalf("preview changed installed subtitle: %v", err)
	}
	if _, err := os.Stat(target + ".kinosail.bak"); !os.IsNotExist(err) {
		t.Fatal("preview created a recovery sidecar")
	}
	assertSubtitleCleanupComparison(t, review)
	assertSubtitleUnpairedComparison(t, importedReview)
	assertSubtitleUnpairedComparison(t, editedReview)
}

func assertSubtitleCleanupComparison(t *testing.T, review subtitlePairingAPIReview) {
	t.Helper()
	expected := []subtitlePairingComparison{{[]int{0}, nil, "removed"}, {[]int{1, 2}, []int{0}, "merged"}, {[]int{3}, []int{1}, "matched"}}
	if !reflect.DeepEqual(review.Comparison, expected) {
		t.Fatalf("cleanup comparison = %#v, want %#v", review.Comparison, expected)
	}
	assertSubtitleCleanupDialogue(t, review)
}

func assertSubtitleCleanupDialogue(t *testing.T, review subtitlePairingAPIReview) {
	t.Helper()
	current := []subtitlePairingCue{{1, 2, "Downloaded from www.example.com"}, {3, 4, "Hello there"}, {4, 5, "Hello there"}, {7, 8, "A later line"}}
	proposed := []subtitlePairingCue{{3.75, 5.75, "Hello there"}, {7.75, 8.75, "A later line"}}
	if !reflect.DeepEqual(review.Current.Cues, current) {
		t.Fatalf("current dialogue/timing = %#v, want %#v", review.Current.Cues, current)
	}
	if !reflect.DeepEqual(review.Proposed.Cues, proposed) {
		t.Fatalf("proposed dialogue/timing = %#v, want %#v", review.Proposed.Cues, proposed)
	}
}

func TestSubtitlePreviewDoesNotLoseRemovedCueInsideMergedGroup(t *testing.T) {
	t.Parallel()
	initial := "1\n00:00:01,000 --> 00:00:02,000\nHello\n\n2\n00:00:02,000 --> 00:00:02,100\nDownloaded from www.example.com\n\n3\n00:00:02,100 --> 00:00:03,000\nHello\n\n4\n00:00:05,000 --> 00:00:06,000\nLater\n"
	handler, base, _ := subtitleInspectorFixture(t, initial)
	response := requestJSON(t, handler, http.MethodPost, base+"/preview", `{"language":"en","removeCredits":true,"mergeRepeated":true}`)
	var review subtitlePairingAPIReview
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &review) != nil {
		t.Fatalf("inner credit preview = %d %s", response.Code, response.Body.String())
	}
	expected := []subtitlePairingComparison{{[]int{0, 2}, []int{0}, "merged"}, {[]int{1}, nil, "removed"}, {[]int{3}, []int{1}, "matched"}}
	if !reflect.DeepEqual(review.Comparison, expected) {
		t.Fatalf("removed credit between merged dialogue was lost: %#v", review.Comparison)
	}
}

func assertSubtitleUnpairedComparison(t *testing.T, review subtitlePairingAPIReview) {
	t.Helper()
	expected := []subtitlePairingComparison{{[]int{0}, nil, "unpaired-current"}, {[]int{1}, nil, "unpaired-current"}, {[]int{2}, nil, "unpaired-current"}, {[]int{3}, nil, "unpaired-current"}, {nil, []int{0}, "unpaired-proposed"}}
	if !reflect.DeepEqual(review.Comparison, expected) {
		t.Fatalf("import fabricates correspondence: %#v, want %#v", review.Comparison, expected)
	}
}

func writeSubtitlePairingBrowserFixture(t *testing.T, handler http.Handler, base string, cleanupJSON, importJSON []byte) {
	t.Helper()
	dir := os.Getenv("KINOSAIL_UI_FIXTURE_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil { //nolint:gosec // The caller explicitly chooses its isolated artifact directory.
		t.Fatal(err)
	}
	files := map[string][]byte{"subtitle-pairing-cleanup.json": cleanupJSON, "subtitle-pairing-import.json": importJSON}
	id := strings.TrimPrefix(base, "/api/v1/subtitle-library/")
	for name, path := range map[string]string{"subtitle-pairing.html": "/subtitles/inspect/" + id, "subtitle-pairing.json": base + "/inspect?language=en", "app.css": "/static/app.css", "subtitle-inspector.css": "/static/subtitle-inspector.css", "subtitle-inspector.js": "/static/subtitle-inspector.js", "theme.js": "/static/theme.js", "icon.svg": "/static/icon.svg", "manrope.woff2": "/static/manrope.woff2"} {
		result := requestApp(t, handler, http.MethodGet, path, "")
		if result.Code != http.StatusOK {
			t.Fatalf("fixture %s = %d", name, result.Code)
		}
		files[name] = result.Body.Bytes()
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil { //nolint:gosec // Fixed artifact filenames under the caller's chosen directory.
			t.Fatal(err)
		}
	}
}
