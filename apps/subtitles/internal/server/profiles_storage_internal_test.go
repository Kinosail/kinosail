package server

import (
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
)

func TestProfileSnapshotsDoNotAliasStoredIdentityState(t *testing.T) {
	store := &profileStore{profiles: []viewerProfile{{
		ID:           "viewer",
		Libraries:    []string{"Movies"},
		SCIMEmails:   []scimProfileEmail{{Value: "viewer@example.com"}},
		Recovery:     []string{"recovery"},
		Passkeys:     []webauthn.Credential{{ID: []byte("credential"), PublicKey: []byte("public-key")}},
		PasskeyUsage: map[string]passkeyUsage{"credential": {Tracked: true}},
	}}}

	snapshots := append(store.list(), mustProfileByID(t, store, "viewer"))
	for _, snapshot := range snapshots {
		snapshot.Libraries[0] = "Changed"
		snapshot.SCIMEmails[0].Value = "changed@example.com"
		snapshot.Recovery[0] = "changed"
		snapshot.Passkeys[0].ID[0] = 'X'
		snapshot.Passkeys[0].PublicKey[0] = 'X'
		snapshot.PasskeyUsage["credential"] = passkeyUsage{}
	}

	stored := store.profiles[0]
	if stored.Libraries[0] != "Movies" || stored.SCIMEmails[0].Value != "viewer@example.com" || stored.Recovery[0] != "recovery" || string(stored.Passkeys[0].ID) != "credential" || string(stored.Passkeys[0].PublicKey) != "public-key" || !stored.PasskeyUsage["credential"].Tracked {
		t.Fatalf("profile snapshot modified stored state: %+v", stored)
	}
}

func mustProfileByID(t *testing.T, store *profileStore, id string) viewerProfile {
	t.Helper()
	profile, found := store.byID(id)
	if !found {
		t.Fatalf("profile %q not found", id)
	}
	return profile
}
