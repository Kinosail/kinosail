package mediashares

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestCreateRejectsEachContentFailure(t *testing.T) {
	t.Parallel()
	for name, items := range map[string][]library.Item{
		"missing": nil, "empty path": {{ID: "item"}},
	} {
		store, state := testStore(t, time.Now(), items...)
		if _, _, err := store.Create([]string{"item"}, time.Hour, 1, true); err == nil || state.data != nil {
			t.Errorf("%s content was accepted", name)
		}
	}
}

func storeWithClaim(now time.Time, token string, expiry int64) *Store {
	shareID := indexedSecret(0)
	return &Store{
		state: State{
			Shares:   map[string]Share{shareID: {ID: shareID, ItemIDs: []string{"item"}, ClaimHash: sessionKey(token), ExpiresAt: expiry, MaxDevices: 1, RightsAcknowledged: true}},
			Sessions: make(map[string]Session),
		},
		persist: func(string, any) error { return nil }, active: make(map[string]int), now: func() time.Time { return now },
		random: bytes.NewReader(bytes.Repeat([]byte{7}, 64)),
	}
}

func TestClaimEnforcesExactExpiryBoundary(t *testing.T) {
	t.Parallel()
	now := time.Unix(100, 0)
	token := strings.Repeat("t", 32)
	if _, _, err := storeWithClaim(now, token, now.Unix()).Claim(token, "Browser"); err == nil {
		t.Fatal("share expiring now was claimed")
	}
	if session, expiry, err := storeWithClaim(now, token, now.Add(time.Second).Unix()).Claim(token, strings.Repeat("x", 80)); err != nil || session == "" || !expiry.Equal(now.Add(time.Second)) {
		t.Fatalf("share after now = %q, %v, %v", session, expiry, err)
	}
}

func TestRevokeRemovesOnlyMatchingSessions(t *testing.T) {
	t.Parallel()
	shareID, otherID := strings.Repeat("s", 32), strings.Repeat("o", 32)
	store := &Store{
		state: State{
			Shares:   map[string]Share{shareID: {ID: shareID}, otherID: {ID: otherID}},
			Sessions: map[string]Session{"matching": {ShareID: shareID}, "other": {ShareID: otherID}},
		},
		persist: func(string, any) error { return nil }, active: make(map[string]int),
	}
	if err := store.Revoke(shareID); err != nil {
		t.Fatal(err)
	}
	if _, found := store.state.Sessions["matching"]; found || store.state.Sessions["other"].ShareID != otherID {
		t.Fatalf("sessions after revoke = %#v", store.state.Sessions)
	}
}

func TestListEnforcesExactExpiryBoundary(t *testing.T) {
	t.Parallel()
	now := time.Unix(100, 0)
	store := &Store{
		state: State{Shares: map[string]Share{
			"now": {ID: "now", ExpiresAt: now.Unix()}, "future": {ID: "future", ExpiresAt: now.Add(time.Second).Unix()},
		}},
		now: func() time.Time { return now },
	}
	views, err := store.List()
	if err != nil || len(views) != 1 || views[0].ID != "future" {
		t.Fatalf("expiry list = %#v, %v", views, err)
	}
}
