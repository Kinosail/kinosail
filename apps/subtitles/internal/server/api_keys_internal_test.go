package server

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestAPIKeysExpireAndTrackUse(t *testing.T) {
	store := newProfileStore(t.TempDir())
	profile, err := newProfile("Owner", "owner-password", true)
	if err != nil || store.addOwner(profile) != nil {
		t.Fatal(err)
	}
	secret, err := store.createAPIKey(profile, "Automation", "library")
	if err != nil {
		t.Fatal(err)
	}
	keyID := sessionKey(secret)
	created := store.apiKeys[keyID]
	if created.ExpiresAt <= created.CreatedAt || created.ExpiresAt-created.CreatedAt > int64((90*24*time.Hour)/time.Second) {
		t.Fatalf("API key lifetime = %+v", created)
	}
	request := httptest.NewRequestWithContext(t.Context(), "GET", "/api/v1/library", nil)
	request.Header.Set("Authorization", "Bearer "+secret)
	if _, ok := store.profile(request); !ok || store.apiKeys[keyID].LastUsed == 0 {
		t.Fatal("active API key was not accepted and tracked")
	}
	expired := store.apiKeys[keyID]
	expired.ExpiresAt = time.Now().Add(-time.Second).Unix()
	store.apiKeys[keyID] = expired
	if _, ok := store.profile(request); ok {
		t.Fatal("expired API key was accepted")
	}
}
