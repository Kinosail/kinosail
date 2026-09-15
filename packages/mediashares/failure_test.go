package mediashares

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func capabilityRequest(t *testing.T, token string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/share/items", nil)
	request.AddCookie(&http.Cookie{Name: "__Host-kinosail_share", Value: token, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	return request
}

func TestStoreCoversFailClosedBranches(t *testing.T) { //nolint:cyclop,funlen,gocognit // This matrix covers independent persistence and capability failures.
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	missing := New("", Dependencies{})
	request := capabilityRequest(t, strings.Repeat("x", 43))
	if _, ok := missing.Item(request, "item"); ok {
		t.Fatal("failed store returned an item")
	}
	if _, ok := missing.Items(request); ok {
		t.Fatal("failed store returned items")
	}
	if _, _, err := missing.Claim(strings.Repeat("x", 32), "Browser"); err == nil {
		t.Fatal("failed store accepted a claim")
	}
	if err := missing.Revoke(strings.Repeat("x", 32)); err == nil || missing.RevokeAll() == nil {
		t.Fatal("failed store accepted a mutation")
	}

	state := new(memoryState)
	dependencies := Dependencies{
		Load: state.load, Persist: state.persist,
		Find:     func(id string) (library.Item, bool) { return library.Item{ID: id, Path: "movie"}, true },
		Snapshot: func() ([]library.Item, error) { return nil, nil }, Now: func() time.Time { return now },
		Random: bytes.NewReader(bytes.Repeat([]byte{3}, 4096)),
	}
	store := New("", dependencies)
	first, firstClaim, err := store.Create([]string{"first"}, 24*time.Hour, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	store.random = bytes.NewReader(bytes.Repeat([]byte{5}, 4096))
	second, _, err := store.Create([]string{"second"}, time.Hour, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := store.List()
	if err != nil || len(listed) != 2 || listed[0].ID != second.ID || listed[1].ID != first.ID {
		t.Fatalf("sorted shares = %#v, %v", listed, err)
	}
	session, expires, err := store.Claim(firstClaim, "Browser")
	if err != nil || !expires.Equal(now.Add(8*time.Hour)) {
		t.Fatalf("bounded claim = %q, %v, %v", session, expires, err)
	}
	store.Release(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	store.Release(capabilityRequest(t, "unknown"))

	store.random = errorReader{}
	if _, _, err := store.Claim(firstClaim, "Browser"); err == nil {
		t.Fatal("claim random failure was ignored")
	}
	store.random = bytes.NewReader(bytes.Repeat([]byte{4}, 64))
	store.persist = func(string, any) error { return errors.New("persist failed") }
	if _, _, err := store.Claim(firstClaim, "Browser"); err == nil {
		t.Fatal("claim persistence failure was ignored")
	}
	if store.RevokeAll() == nil {
		t.Fatal("revoke-all persistence failure was ignored")
	}

	tooMany, _ := testStore(t, now, library.Item{ID: "item", Path: "movie"})
	for index := range 256 {
		id := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{byte(index)}, 32))
		tooMany.state.Shares[id] = Share{ID: id, ExpiresAt: now.Add(time.Hour).Unix()}
	}
	if _, _, err := tooMany.Create([]string{"item"}, time.Hour, 1, true); err == nil {
		t.Fatal("share capacity was bypassed")
	}
}

func TestNewAndStateValidationFailureBranches(t *testing.T) { //nolint:cyclop,funlen // Each case proves one persisted-state boundary.
	now := time.Now().Unix()
	secret := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	hash := strings.Repeat("0", 64)
	validShare := Share{ID: secret, ItemIDs: []string{"item"}, ClaimHash: hash, ExpiresAt: now + 60, MaxDevices: 1, RightsAcknowledged: true}
	valid := State{Shares: map[string]Share{secret: validShare}, Sessions: map[string]Session{}}
	invalidSession := CloneState(valid)
	invalidSession.Sessions[hash] = Session{ShareID: "missing", ExpiresAt: now + 30}
	duplicateItem := CloneState(valid)
	share := duplicateItem.Shares[secret]
	share.ItemIDs = []string{"item", "item"}
	duplicateItem.Shares[secret] = share
	emptyItems := CloneState(valid)
	share = emptyItems.Shares[secret]
	share.ItemIDs = nil
	emptyItems.Shares[secret] = share
	for name, state := range map[string]State{"invalid session": invalidSession, "duplicate item": duplicateItem, "empty items": emptyItems} {
		if ValidState(state) {
			t.Fatalf("%s state was accepted", name)
		}
	}

	loaded := &memoryState{data: []byte(`{}`)}
	store := New(t.TempDir(), Dependencies{
		Load: loaded.load, Persist: loaded.persist,
		Find:     func(string) (library.Item, bool) { return library.Item{}, false },
		Snapshot: func() ([]library.Item, error) { return nil, nil },
	})
	if store.state.Shares == nil || store.state.Sessions == nil {
		t.Fatal("nil loaded maps were not initialized")
	}
	if _, err := store.List(); err != nil {
		t.Fatalf("valid loaded state failed: %v", err)
	}
	loaded.data = []byte(`{"shares":{"bad":{"id":"bad"}},"sessions":{}}`)
	if _, err := New(t.TempDir(), Dependencies{Load: loaded.load, Persist: loaded.persist, Find: store.find, Snapshot: store.snapshot}).List(); err == nil {
		t.Fatal("invalid loaded state was accepted")
	}
}
