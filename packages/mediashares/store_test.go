package mediashares

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type memoryState struct {
	data       []byte
	loadErr    error
	persistErr error
}

func (state *memoryState) load(_ string, target any) (bool, error) {
	if state.loadErr != nil || state.data == nil {
		return false, state.loadErr
	}
	return true, json.Unmarshal(state.data, target)
}

func (state *memoryState) persist(_ string, value any) error {
	if state.persistErr != nil {
		return state.persistErr
	}
	data, err := json.Marshal(value)
	state.data = data
	return err
}

func testStore(t *testing.T, now time.Time, items ...library.Item) (*Store, *memoryState) {
	t.Helper()
	state := new(memoryState)
	byID := make(map[string]library.Item, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	store := New(t.TempDir(), Dependencies{
		Load: state.load, Persist: state.persist,
		Find:     func(id string) (library.Item, bool) { item, found := byID[id]; return item, found },
		Snapshot: func() ([]library.Item, error) { return append([]library.Item(nil), items...), nil },
		Now:      func() time.Time { return now }, Random: bytes.NewReader(bytes.Repeat([]byte{7}, 4096)),
	})
	return store, state
}

func TestStoreCapabilityLifecycle(t *testing.T) { //nolint:cyclop,gocognit // One lifecycle proves every capability transition.
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	directory := t.TempDir()
	media := filepath.Join(directory, "movie.mp4")
	if err := os.WriteFile(media, []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, _ := testStore(t, now, library.Item{ID: "item", Path: media})
	share, claim, err := store.Create([]string{"item"}, time.Hour, 1, true)
	if err != nil || share.ID == "" || claim == "" {
		t.Fatalf("create = %#v, %q, %v", share, claim, err)
	}
	listed, err := store.List()
	if err != nil || len(listed) != 1 || listed[0].ID != share.ID {
		t.Fatalf("list = %#v, %v", listed, err)
	}
	withoutCookie := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/share/items", nil)
	if _, ok := store.Items(withoutCookie); ok {
		t.Fatal("missing capability listed items")
	}
	session, expires, err := store.Claim(claim, "Firefox/123")
	if err != nil || !expires.Equal(now.Add(time.Hour)) {
		t.Fatalf("claim = %q, %v, %v", session, expires, err)
	}
	if _, _, err := store.Claim(claim, "Second"); !errors.Is(err, ErrDeviceLimit) {
		t.Fatalf("device limit = %v", err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/share/media/item", nil)
	request.SetPathValue("id", "item")
	request.AddCookie(&http.Cookie{Name: "__Host-kinosail_share", Value: session, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	if items, ok := store.Items(request); !ok || len(items) != 1 || items[0].ID != "item" {
		t.Fatalf("items = %#v, %v", items, ok)
	}
	if path, ok := store.Item(request, "item"); !ok || path != media {
		t.Fatalf("item = %q, %v", path, ok)
	}
	if _, ok := store.Item(request, "item"); ok {
		t.Fatal("active stream limit was bypassed")
	}
	store.Release(request)
	response := httptest.NewRecorder()
	store.Serve(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "movie" || response.Header().Get("Cache-Control") != "no-store, private" {
		t.Fatalf("serve = %d, %q, %q", response.Code, response.Body.String(), response.Header().Get("Cache-Control"))
	}
	if err := store.Revoke(share.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Items(request); ok {
		t.Fatal("revoked grant remained active")
	}
}

func TestStoreRejectsUnsafeCreationBeforePersistence(t *testing.T) {
	t.Parallel()
	now := time.Now()
	validItems := []string{"item"}
	tooMany := make([]string, 101)
	for index := range tooMany {
		tooMany[index] = "item-" + string(rune(index+32))
	}
	for name, request := range map[string]struct {
		items        []string
		lifetime     time.Duration
		devices      int
		acknowledged bool
	}{
		"rights": {validItems, time.Hour, 1, false}, "empty": {nil, time.Hour, 1, true}, "many": {tooMany, time.Hour, 1, true},
		"short": {validItems, time.Second, 1, true}, "long": {validItems, 25 * time.Hour, 1, true}, "devices low": {validItems, time.Hour, 0, true}, "devices high": {validItems, time.Hour, 9, true},
		"unknown": {[]string{"missing"}, time.Hour, 1, true}, "duplicate": {[]string{"item", "item"}, time.Hour, 1, true},
	} {
		t.Run(name, func(t *testing.T) {
			store, state := testStore(t, now, library.Item{ID: "item", Path: "movie.mp4"})
			if _, _, err := store.Create(request.items, request.lifetime, request.devices, request.acknowledged); err == nil || state.data != nil {
				t.Fatalf("unsafe creation = %v, persisted %q", err, state.data)
			}
		})
	}
}

func TestStoreFailsClosedOnDependencyRandomAndPersistenceErrors(t *testing.T) {
	t.Parallel()
	missing := New("", Dependencies{})
	if _, err := missing.List(); err == nil {
		t.Fatal("missing dependencies were accepted")
	}
	if _, _, err := missing.Create([]string{"item"}, time.Hour, 1, true); err == nil {
		t.Fatal("missing dependencies were accepted for creation")
	}
	now := time.Now()
	state := new(memoryState)
	dependencies := Dependencies{Load: state.load, Persist: state.persist, Find: func(string) (library.Item, bool) { return library.Item{ID: "item", Path: "movie"}, true }, Snapshot: func() ([]library.Item, error) { return nil, nil }, Now: func() time.Time { return now }, Random: errorReader{}}
	if _, _, err := New("", dependencies).Create([]string{"item"}, time.Hour, 1, true); err == nil {
		t.Fatal("random failure was ignored")
	}
	dependencies.Random = bytes.NewReader(make([]byte, 32))
	if _, _, err := New("", dependencies).Create([]string{"item"}, time.Hour, 1, true); err == nil {
		t.Fatal("second random failure was ignored")
	}
	state.persistErr = errors.New("persist failed")
	dependencies.Random = bytes.NewReader(make([]byte, 64))
	if _, _, err := New("", dependencies).Create([]string{"item"}, time.Hour, 1, true); err == nil {
		t.Fatal("persistence failure was ignored")
	}
	state.loadErr = errors.New("load failed")
	if _, err := New("state", dependencies).List(); err == nil {
		t.Fatal("load failure was ignored")
	}
}

func TestClaimRevokeAndDeliveryFailurePaths(t *testing.T) { //nolint:cyclop // A compact matrix covers fail-closed capability reads.
	t.Parallel()
	now := time.Now()
	store, _ := testStore(t, now, library.Item{ID: "item", Path: "movie"})
	if _, _, err := store.Claim("short", "browser"); err == nil {
		t.Fatal("short claim accepted")
	}
	if _, _, err := store.Claim(strings.Repeat("x", 32), strings.Repeat("x", 81)); err == nil {
		t.Fatal("long device accepted")
	}
	if _, _, err := store.Claim(strings.Repeat("x", 32), "browser"); err == nil {
		t.Fatal("unknown claim accepted")
	}
	if err := store.Revoke("short"); err == nil {
		t.Fatal("short revoke accepted")
	}
	if err := store.Revoke(base64.RawURLEncoding.EncodeToString(make([]byte, 32))); err == nil {
		t.Fatal("unknown revoke accepted")
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/share/media/item", nil)
	request.SetPathValue("id", "item")
	request.Header.Set("Range", "bytes=0-1,3-4")
	response := httptest.NewRecorder()
	store.Serve(response, request)
	if response.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("invalid range = %d", response.Code)
	}
	request.Header.Del("Range")
	response = httptest.NewRecorder()
	store.Serve(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unauthorized media = %d", response.Code)
	}
	store, _ = testStore(t, now, library.Item{ID: "item", Path: "movie"})
	share, _, err := store.Create([]string{"item"}, time.Hour, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	store.persist = func(string, any) error { return errors.New("persist failed") }
	if err := store.Revoke(share.ID); err == nil {
		t.Fatal("revoke persistence failure ignored")
	}
}

func TestStateValidationClonePruneAndRevokeAll(t *testing.T) { //nolint:cyclop // One state matrix covers independent validation branches.
	t.Parallel()
	secret := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	hash := strings.Repeat("0", 64)
	now := time.Now().Unix()
	valid := State{Shares: map[string]Share{secret: {ID: secret, ItemIDs: []string{"item"}, ClaimHash: hash, ExpiresAt: now + 60, MaxDevices: 1, RightsAcknowledged: true}}, Sessions: map[string]Session{hash: {ShareID: secret, ExpiresAt: now + 30, Device: "Browser"}}}
	if !ValidState(valid) || !ValidSecret(secret) || !ValidHash(hash) {
		t.Fatal("valid state rejected")
	}
	clone := CloneState(valid)
	clone.Shares[secret] = Share{}
	if valid.Shares[secret].ID == "" {
		t.Fatal("clone mutated source")
	}
	invalid := CloneState(valid)
	invalid.Shares[secret] = Share{ID: "bad"}
	tooMany := State{Shares: make(map[string]Share), Sessions: make(map[string]Session)}
	for index := range 257 {
		tooMany.Shares[string(rune(index))] = Share{}
	}
	if ValidState(invalid) || ValidState(tooMany) {
		t.Fatal("invalid state accepted")
	}
	Prune(&valid, now+120)
	if len(valid.Shares) != 0 || len(valid.Sessions) != 0 {
		t.Fatalf("pruned state = %#v", valid)
	}
	store, _ := testStore(t, time.Now(), library.Item{ID: "item", Path: "movie"})
	if _, _, err := store.Create([]string{"item"}, time.Hour, 1, true); err != nil || store.RevokeAll() != nil {
		t.Fatal(err)
	}
	if listed, err := store.List(); err != nil || len(listed) != 0 {
		t.Fatalf("revoked all = %#v, %v", listed, err)
	}
}

func TestRangeAndDeviceNormalization(t *testing.T) {
	t.Parallel()
	for value, valid := range map[string]bool{"bytes=0-": true, "bytes=-10": true, "bytes=1-2": true, "": false, "items=0-1": false, strings.Repeat("x", 129): false} {
		if ValidSingleByteRange(value) != valid {
			t.Errorf("range %q validity = %v", value, !valid)
		}
	}
	for _, test := range []struct{ value, want string }{{"", "Web browser"}, {" Firefox/123 ", "Firefox"}, {"Edg/1", "Microsoft Edge"}, {"Chrome/1", "Chrome"}, {"Safari/1", "Safari"}, {strings.Repeat("x", 81), strings.Repeat("x", 80)}, {"Device", "Device"}} {
		if got := cleanDeviceName(test.value); got != test.want {
			t.Errorf("device %q = %q", test.value, got)
		}
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
