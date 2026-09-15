package server

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/federation"
)

func TestSAMLIdentityLinkingRejectsConflictsAndPersistsValidLinks(t *testing.T) { //nolint:cyclop,funlen // One lifecycle contract covers invalid, conflict, ownership, and persistence paths.
	store := &profileStore{
		file:    "profiles.json",
		persist: func(string, any) error { return nil },
		profiles: []viewerProfile{
			{ID: "viewer", Name: "Viewer"},
			{ID: "linked", Name: "Linked", SAMLIssuer: "https://idp.example", SAMLSubject: "linked-subject"},
			{ID: "managed", Name: "Managed", SCIMManaged: true},
		},
	}
	for name, want := range map[string]string{
		"missing issuer":  "SAML identity is invalid",
		"missing subject": "SAML identity is invalid",
		"duplicate":       "SAML identity is already linked",
		"managed":         "SCIM-managed",
		"missing profile": "viewer profile was not found",
	} {
		var id, issuer, subject string
		switch name {
		case "missing issuer":
			id, subject = "viewer", "subject"
		case "missing subject":
			id, issuer = "viewer", "https://idp.example"
		case "duplicate":
			id, issuer, subject = "viewer", "https://idp.example", "linked-subject"
		case "managed":
			id, issuer, subject = "managed", "https://idp.example", "managed-subject"
		case "missing profile":
			id, issuer, subject = "missing", "https://idp.example", "missing-subject"
		}
		t.Run(name, func(t *testing.T) {
			if err := store.federatedProfiles().Link(federation.SAMLProtocol, id, federation.Identity{Issuer: issuer, Subject: subject}); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("linkSAML error = %v, want %q", err, want)
			}
		})
	}
	identity := federation.Identity{Issuer: "https://idp.example", Subject: "viewer-subject"}
	if err := store.federatedProfiles().Link(federation.SAMLProtocol, "viewer", identity); err != nil {
		t.Fatalf("valid link: %v", err)
	}
	if profile, found := store.federatedProfiles().Find(federation.SAMLProtocol, identity); !found || profile.ID != "viewer" {
		t.Fatalf("linked profile = %+v, found=%v", profile, found)
	}
	if err := store.federatedProfiles().Unlink(federation.SAMLProtocol, "viewer"); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if _, found := store.federatedProfiles().Find(federation.SAMLProtocol, identity); found {
		t.Fatal("unlinked SAML identity remained active")
	}
}

func TestSCIMSAMLIdentityAutoLinkingRequiresOneActiveStableMatch(t *testing.T) { //nolint:cyclop,funlen // One contract covers empty, missing, ambiguous, conflicting, and valid matches.
	identity := federation.Identity{Issuer: "https://idp.example", Subject: "directory-1"}
	store := &profileStore{file: "profiles.json", persist: func(string, any) error { return nil }}
	profiles := store.federatedProfiles()
	if _, found, err := profiles.AutoLinkSCIM(federation.SAMLProtocol, federation.Identity{}); err != nil || found {
		t.Fatalf("empty identity linked: found=%v err=%v", found, err)
	}
	store.profiles = []viewerProfile{{ID: "viewer", SCIMManaged: true, SCIMExternalID: "other"}}
	if _, found, err := profiles.AutoLinkSCIM(federation.SAMLProtocol, identity); err != nil || found {
		t.Fatalf("missing match linked: found=%v err=%v", found, err)
	}
	store.profiles = []viewerProfile{{ID: "deleted", SCIMManaged: true, SCIMDeleted: true, SCIMExternalID: identity.Subject}}
	if _, found, err := profiles.AutoLinkSCIM(federation.SAMLProtocol, identity); err != nil || found {
		t.Fatalf("deleted match linked: found=%v err=%v", found, err)
	}
	store.profiles = []viewerProfile{{ID: "first", SCIMManaged: true, SCIMExternalID: identity.Subject}, {ID: "second", SCIMManaged: true, SCIMExternalID: identity.Subject}}
	if _, found, err := profiles.AutoLinkSCIM(federation.SAMLProtocol, identity); err != nil || found {
		t.Fatalf("ambiguous match linked: found=%v err=%v", found, err)
	}
	store.profiles = []viewerProfile{{ID: "match", SCIMManaged: true, SCIMExternalID: identity.Subject}, {ID: "conflict", SAMLIssuer: identity.Issuer, SAMLSubject: identity.Subject}}
	if _, found, err := profiles.AutoLinkSCIM(federation.SAMLProtocol, identity); err != nil || found {
		t.Fatalf("conflicting match linked: found=%v err=%v", found, err)
	}
	store.profiles = []viewerProfile{{ID: "match", SCIMManaged: true, SCIMExternalID: identity.Subject}}
	profile, found, err := profiles.AutoLinkSCIM(federation.SAMLProtocol, identity)
	if err != nil || !found || profile.ID != "match" || profile.SAMLIssuer != identity.Issuer || profile.SAMLSubject != identity.Subject {
		t.Fatalf("valid match = %+v, found=%v err=%v", profile, found, err)
	}
}
