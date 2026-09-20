package server

import (
	"encoding/json"
	"errors"
	"math"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestProgressSyncSnapshotBounds(t *testing.T) {
	t.Parallel()
	valid := mediaProgressSnapshot{Seconds: 31_536_000, Session: strings.Repeat("s", 128), Revision: 9_007_199_254_740_991}
	if !validSyncSnapshot(valid, true) || !validSyncSnapshot(mediaProgressSnapshot{}, false) {
		t.Fatal("valid boundary rejected")
	}
	for name, change := range map[string]func(*mediaProgressSnapshot){
		"negative":         func(s *mediaProgressSnapshot) { s.Seconds = -1 },
		"too long":         func(s *mediaProgressSnapshot) { s.Seconds++ },
		"nan":              func(s *mediaProgressSnapshot) { s.Seconds = math.NaN() },
		"infinity":         func(s *mediaProgressSnapshot) { s.Seconds = math.Inf(1) },
		"session size":     func(s *mediaProgressSnapshot) { s.Session += "s" },
		"control":          func(s *mediaProgressSnapshot) { s.Session = "bad\n" },
		"revision size":    func(s *mediaProgressSnapshot) { s.Revision++ },
		"missing session":  func(s *mediaProgressSnapshot) { s.Session = "" },
		"missing revision": func(s *mediaProgressSnapshot) { s.Revision = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			value := valid
			change(&value)
			if validSyncSnapshot(value, true) {
				t.Fatal("invalid snapshot accepted")
			}
		})
	}
}

func TestProgressSyncHTTPRejectsWithoutPersistence(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{`, `{}`, `{"expected":{"seconds":0,"watched":false,"session":"","revision":0}}`, `{"progress":{}}`, `{"progress":{"seconds":-1,"session":"s","revision":1},"expected":{"seconds":0,"watched":false,"session":"","revision":0}}`, `{"progress":{"session":"s","revision":1},"expected":{"revision":9007199254740992}}`, `{"progress":{"session":"s","revision":1,"unknown":true},"expected":{"seconds":0,"watched":false,"session":"","revision":0}}`, `{"progress":{"session":"s","revision":1},"expected":{"seconds":0,"watched":false,"session":"","revision":0},"playbackToken":"bad"}`} {
		store := newProgressStore(t.TempDir())
		writes := 0
		store.SetPersistence(func(string, any) error { writes++; return nil })
		request := httptest.NewRequestWithContext(withViewer(t.Context(), viewerProfile{ID: "viewer", Owner: true}), "PUT", "/", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.SetPathValue("id", "aaaaaaaaaaaaaaaa")
		response := httptest.NewRecorder()
		syncMediaProgress(progressSyncIndex(t), store)(response, request)
		if response.Code != 400 || writes != 0 {
			t.Fatalf("body %s: status %d writes %d", body, response.Code, writes)
		}
	}
}

func TestProgressSyncHTTPRetriesConflictsAndPersistence(t *testing.T) {
	t.Parallel()
	store := newProgressStore(t.TempDir())
	index := progressSyncIndex(t)
	send := func(id, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequestWithContext(withViewer(t.Context(), viewerProfile{ID: "viewer", Owner: true}), "PUT", "/", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.SetPathValue("id", id)
		response := httptest.NewRecorder()
		syncMediaProgress(index, store)(response, request)
		return response
	}
	first := `{"progress":{"seconds":42,"watched":false,"session":"first","revision":1},"expected":{"seconds":0,"watched":false,"session":"","revision":0}}`
	for range 2 {
		if response := send("aaaaaaaaaaaaaaaa", first); response.Code != 200 {
			t.Fatalf("save/retry: %d %s", response.Code, response.Body)
		}
	}
	if response := send("missing", first); response.Code != 404 {
		t.Fatalf("missing item: %d", response.Code)
	}
	if response := send("aaaaaaaaaaaaaaaa", `{"progress":{"seconds":9,"watched":false,"session":"second","revision":1},"expected":{"seconds":0,"watched":false,"session":"","revision":0}}`); response.Code != 409 {
		t.Fatalf("conflict: %d", response.Code)
	}
	store.SetPersistence(func(string, any) error { return errors.New("unavailable") })
	response := send("aaaaaaaaaaaaaaaa", `{"progress":{"seconds":50,"watched":false,"session":"first","revision":2},"expected":{"seconds":0,"watched":false,"session":"","revision":0}}`)
	if response.Code != 500 {
		t.Fatalf("persistence: %d", response.Code)
	}
	response = send("aaaaaaaaaaaaaaaa", first)
	var state playbackState
	if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if state.Seconds != 42 || state.Revision != 1 {
		t.Fatalf("failed write changed state: %#v", state)
	}
}

func TestSynchronizedProgressRequiresMatchingSnapshotForNewSession(t *testing.T) {
	t.Parallel()
	state := playbackState{Seconds: 42, Session: "old", Revision: 3, Watched: true}
	expected := mediaProgressSnapshot{Seconds: 42, Session: "old", Revision: 3, Watched: true}
	update := mediaProgressSnapshot{Seconds: 50, Session: "new", Revision: 1}
	for name, change := range map[string]func(*mediaProgressSnapshot){
		"session":  func(s *mediaProgressSnapshot) { s.Session = "other" },
		"revision": func(s *mediaProgressSnapshot) { s.Revision++ },
		"seconds":  func(s *mediaProgressSnapshot) { s.Seconds++ },
		"watched":  func(s *mediaProgressSnapshot) { s.Watched = false },
	} {
		t.Run(name, func(t *testing.T) {
			bad := expected
			change(&bad)
			got, accepted, err := synchronizedProgress(50, bad, update)(state)
			if err != nil || accepted || !reflect.DeepEqual(got, state) {
				t.Fatalf("conflicting update: %#v %v %v", got, accepted, err)
			}
		})
	}
	got, accepted, err := synchronizedProgress(50, expected, update)(state)
	if err != nil || !accepted || got.Seconds != 50 || got.Session != "new" {
		t.Fatalf("matching update: %#v %v %v", got, accepted, err)
	}
}

func progressSyncIndex(t *testing.T) *libraryIndex {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "film.mp4")
	if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	index := memoryLibraryIndex([]library.Item{{ID: "aaaaaaaaaaaaaaaa", Kind: "video", Path: path}}, true)
	index.SetRoots([]libraryRoot{{Path: root}})
	return index
}
