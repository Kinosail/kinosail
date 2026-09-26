package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMediaPreferencesRejectInvalidWithoutSaving(t *testing.T) {
	base, _ := json.Marshal(defaultMediaPreferences())
	for _, key := range []string{"playback", "autoDownloadNext", "removeWatched", "downloadLimitGiB", "wifiOnly", "readerFontSize", "readerTheme"} {
		t.Run("missing-"+key, func(t *testing.T) {
			var fields map[string]json.RawMessage
			_ = json.Unmarshal(base, &fields)
			delete(fields, key)
			raw, _ := json.Marshal(fields)
			assertMediaPreferenceRejected(t, string(raw))
		})
	}
	for _, replacement := range []struct{ old, new string }{
		{`"rate":1`, `"rate":0`},
		{`"volumeBoost":1`, `"volumeBoost":3`},
		{`"autoDownloadNext":0`, `"autoDownloadNext":4`},
		{`"wifiOnly":true`, `"wifiOnly":null`},
		{`"downloadLimitGiB":20`, `"downloadLimitGiB":8388608`},
		{`"readerTheme":"auto"`, `"readerTheme":"unknown"`},
	} {
		t.Run(replacement.new, func(t *testing.T) {
			assertMediaPreferenceRejected(t, strings.Replace(string(base), replacement.old, replacement.new, 1))
		})
	}
	assertMediaPreferenceRejected(t, `{}`)
	assertMediaPreferenceRejected(t, strings.TrimSuffix(string(base), "}")+`,"unexpected":true}`)
}

func assertMediaPreferenceRejected(t *testing.T, raw string) {
	t.Helper()
	store := newMediaExperienceStore("", nil)
	before, _ := json.Marshal(store.value)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/me/media-preferences", strings.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	store.defaultHandler()(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	after, _ := json.Marshal(store.value)
	if string(before) != string(after) {
		t.Fatal("rejected preferences changed state")
	}
}

func TestMediaPreferencesJSONRequiresPlaybackFields(t *testing.T) {
	raw, _ := json.Marshal(defaultMediaPreferences().Playback)
	for _, key := range []string{"rate", "volumeBoost", "nightMode", "dialogueBoost", "audioLanguage", "subtitleLanguage", "audioTrack", "subtitleTrack"} {
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(raw, &fields)
		delete(fields, key)
		missing, _ := json.Marshal(fields)
		var value playbackPreferences
		if json.Unmarshal(missing, &value) == nil {
			t.Fatalf("accepted missing %s", key)
		}
	}
}

func TestMediaPreferenceStorePersistsViewerIsolation(t *testing.T) {
	dir := t.TempDir()
	store := newMediaExperienceStore(dir, nil)
	value := defaultMediaPreferences()
	value.Playback.Rate = 1.5
	if err := store.change(func(next *mediaExperienceState) error {
		next.Defaults[experienceKey("viewer-a", "defaults")] = value
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	restored := newMediaExperienceStore(dir, nil)
	if restored.defaults("viewer-a").Playback.Rate != 1.5 || restored.defaults("viewer-b").Playback.Rate != 1 {
		t.Fatal("preferences crossed viewers or failed to persist")
	}
}

func TestSyncSnapshotRequiresAllFields(t *testing.T) {
	for _, raw := range []string{`{}`, `{"seconds":0,"session":"s","revision":1}`, `{"seconds":0,"watched":null,"session":"s","revision":1}`, `{"seconds":0,"watched":false,"session":"s","revision":1,"unknown":true}`} {
		var snapshot mediaProgressSnapshot
		if json.Unmarshal([]byte(raw), &snapshot) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestMediaDownloadStorageLimits(t *testing.T) {
	for _, limit := range []int{0, 50, 250, 1000, 8388607} {
		value := defaultMediaPreferences()
		value.DownloadLimitGiB = limit
		raw, _ := json.Marshal(value)
		store := newMediaExperienceStore("", nil)
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/me/media-preferences", strings.NewReader(string(raw)))
		response := httptest.NewRecorder()
		store.defaultHandler()(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("limit %d: %d %s", limit, response.Code, response.Body.String())
		}
		var saved mediaPreferences
		if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil || saved.DownloadLimitGiB != limit {
			t.Fatalf("limit did not round trip: %s", response.Body.String())
		}
	}
	base, _ := json.Marshal(defaultMediaPreferences())
	for _, invalid := range []string{"-1", "0.5", "null", `"0"`, "8388608", "99999999999999999999999999999999999"} {
		assertMediaPreferenceRejected(t, strings.Replace(string(base), `"downloadLimitGiB":20`, `"downloadLimitGiB":`+invalid, 1))
	}
}
