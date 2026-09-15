package server

import (
	"errors"
	"strings"
	"testing"
	"time"

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

func TestSCIMRehydrationRejectsAmbiguousDeletedProfiles(t *testing.T) {
	now := time.Now().UTC()
	deleted := func(id, name string) viewerProfile {
		return viewerProfile{ID: id, Name: name, SCIMManaged: true, SCIMDeleted: true, Disabled: true, SCIMUserName: "same@example.com", SCIMName: scimProfileName{Formatted: name}, SCIMCreatedAt: now, SCIMUpdatedAt: now}
	}
	store := &profileStore{
		profiles:    []viewerProfile{{ID: "owner", Name: "Owner", Owner: true}, deleted("deleted-1", "First"), deleted("deleted-2", "Second")},
		sessions:    map[string]viewerSession{},
		apiKeys:     map[string]apiKey{},
		file:        "profiles.json",
		sessionFile: "sessions.json",
		apiFile:     "api-keys.json",
		persist:     func(string, any) error { return nil },
	}
	if _, err := store.createSCIMProfile(scimProfileInput{UserName: "same@example.com", Name: "Recreated", Active: true}); !errors.Is(err, errSCIMConflict) {
		t.Fatalf("ambiguous SCIM rehydration error = %v", err)
	}
}

func TestSCIMOIDCLinkingRequiresOneStableProvisionedSubject(t *testing.T) { //nolint:cyclop // One lifecycle test proves all stable-subject link outcomes.
	identity := oidcIdentity{Issuer: "https://issuer.example", Subject: "subject-1"}
	store := &profileStore{file: "profiles.json", persist: func(string, any) error { return nil }, profiles: []viewerProfile{{ID: "scim-1", Name: "Viewer", SCIMManaged: true, SCIMUserName: "viewer@example.com", SCIMExternalID: "directory-1"}}}
	profiles := store.federatedProfiles()
	if _, found, err := profiles.AutoLinkSCIM(federation.OIDCProtocol, identity); err != nil || found || store.profiles[0].OIDCSubject != "" {
		t.Fatalf("unstable SCIM identifier linked found=%v subject=%q err=%v", found, store.profiles[0].OIDCSubject, err)
	}
	store.profiles[0].SCIMExternalID = identity.Subject
	if profile, found, err := profiles.AutoLinkSCIM(federation.OIDCProtocol, identity); err != nil || !found || profile.OIDCSubject != identity.Subject {
		t.Fatalf("stable SCIM subject link found=%v profile=%+v err=%v", found, profile, err)
	}
	store.profiles = []viewerProfile{{ID: "scim-1", SCIMManaged: true, SCIMExternalID: identity.Subject}, {ID: "scim-2", SCIMManaged: true, SCIMExternalID: identity.Subject}}
	if _, found, err := profiles.AutoLinkSCIM(federation.OIDCProtocol, identity); err != nil || found || store.profiles[0].OIDCSubject != "" || store.profiles[1].OIDCSubject != "" {
		t.Fatalf("ambiguous SCIM subjects linked found=%v profiles=%+v err=%v", found, store.profiles, err)
	}
}
