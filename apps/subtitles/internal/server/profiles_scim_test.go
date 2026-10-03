package server

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/federation"
)

func TestSCIMManagedProfileRejectsLocalPassword(t *testing.T) {
	store := &profileStore{profiles: []viewerProfile{{ID: "scim-profile", SCIMManaged: true, Credential: ""}}}
	if err := store.resetPassword("scim-profile", "scim-secure-password-0123"); err == nil || !strings.Contains(err.Error(), "SCIM-managed") {
		t.Fatalf("resetPassword(SCIM-managed) error = %v", err)
	}
	if store.profiles[0].Credential != "" {
		t.Fatal("SCIM-managed profile received a local credential")
	}
}

func TestSCIMManagedProfileAllowsLocalPolicyButRejectsIdentityLifecycleChanges(t *testing.T) {
	store := &profileStore{file: "profiles.json", sessionFile: "sessions.json", apiFile: "api_keys.json", profiles: []viewerProfile{{ID: "scim-profile", SCIMManaged: true}}, sessions: map[string]viewerSession{}, persist: func(string, any) error { return nil }}
	if err := store.setProfile("scim-profile", false, profilePolicy{Rating: "all", Libraries: []string{"all"}}); err != nil {
		t.Fatalf("setProfile(SCIM-managed) error = %v", err)
	}
	if !contains(store.profiles[0].Libraries, "all") || store.profiles[0].Rating != "all" {
		t.Fatalf("SCIM-managed profile policy = %+v", store.profiles[0])
	}
	if err := store.removeProfile("scim-profile"); err == nil || !strings.Contains(err.Error(), "SCIM-managed") {
		t.Fatalf("removeProfile(SCIM-managed) error = %v", err)
	}
	if err := store.federatedProfiles().Link(federation.OIDCProtocol, "scim-profile", federation.Identity{Issuer: "https://issuer.example", Subject: "subject"}); err == nil || !strings.Contains(err.Error(), "SCIM-managed") {
		t.Fatalf("linkOIDC(SCIM-managed) error = %v", err)
	}
	if len(store.profiles) != 1 || !store.profiles[0].SCIMManaged {
		t.Fatal("SCIM-managed profile was changed by local lifecycle operation")
	}
}
