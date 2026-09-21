package server

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestItemPlaybackPreferencesIsolationRemovalAndPersistenceFailure(t *testing.T) {
	t.Parallel()
	store := newMediaExperienceStore(t.TempDir(), nil)
	index := progressSyncIndex(t)
	send := func(viewer, method, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequestWithContext(withViewer(t.Context(), viewerProfile{ID: viewer, Owner: true}), method, "/", strings.NewReader(body))
		request.SetPathValue("id", "aaaaaaaaaaaaaaaa")
		response := httptest.NewRecorder()
		store.itemHandler(index)(response, request)
		return response
	}
	value := defaultMediaPreferences().Playback
	value.Rate = 1.5
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	assertPlaybackOverride(t, send("first", "PUT", string(raw)), true, 1.5)
	assertPlaybackOverride(t, send("first", "GET", ""), true, 1.5)
	assertPlaybackOverride(t, send("other", "GET", ""), false, 1)
	store.persist = func(string, any) error { return errors.New("blocked") }
	if response := send("first", "DELETE", ""); response.Code != 500 {
		t.Fatalf("failed save=%d", response.Code)
	}
	assertPlaybackOverride(t, send("first", "GET", ""), true, 1.5)
	store.persist = func(string, any) error { return nil }
	assertPlaybackOverride(t, send("first", "DELETE", ""), false, 1)
	for _, body := range []string{`{}`, strings.Replace(string(raw), `"rate":1.5`, `"rate":0`, 1)} {
		if response := send("first", "PUT", body); response.Code != 400 {
			t.Fatalf("invalid preferences=%d", response.Code)
		}
	}
	assertPlaybackOverride(t, send("first", "GET", ""), false, 1)
}

func assertPlaybackOverride(t *testing.T, response *httptest.ResponseRecorder, overridden bool, rate float64) {
	t.Helper()
	var result struct {
		Playback   playbackPreferences `json:"playback"`
		Overridden bool                `json:"overridden"`
	}
	if response.Code != 200 {
		t.Fatalf("preferences=%d %s", response.Code, response.Body)
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Overridden != overridden || result.Playback.Rate != rate {
		t.Fatalf("preferences=%#v want override=%v rate=%v", result, overridden, rate)
	}
}
