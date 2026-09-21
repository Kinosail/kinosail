package federation

import (
	"errors"
	"slices"
	"sync"
	"testing"
)

type linkedProfile struct {
	ID, External string
	OIDC, SAML   Identity
	Managed      bool
	Disabled     bool
	Deleted      bool
}

func TestProfilesValidateBeforePersistence(t *testing.T) {
	t.Parallel()
	values := []linkedProfile{{ID: "viewer"}, {ID: "other", OIDC: Identity{Issuer: "issuer", Subject: "used"}}}
	profiles, persisted := linkedProfiles(&values, nil)
	for name, identity := range map[string]Identity{
		"missing issuer":  {Subject: "subject"},
		"invalid subject": {Issuer: "issuer", Subject: "bad\nsubject"},
		"duplicate":       {Issuer: "issuer", Subject: "used"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := profiles.Link(OIDCProtocol, "viewer", identity); err == nil {
				t.Fatal("invalid identity link was accepted")
			}
		})
	}
	if *persisted != 0 || values[0].OIDC != (Identity{}) {
		t.Fatalf("invalid identity caused effects: persisted=%d profile=%#v", *persisted, values[0])
	}
}

func TestProfilesEnforceOwnershipPersistenceAndLookup(t *testing.T) {
	t.Parallel()
	identity := Identity{Issuer: "issuer", Subject: "subject"}
	values := []linkedProfile{{ID: "viewer", Managed: true}}
	profiles, _ := linkedProfiles(&values, nil)
	if err := profiles.Link(SAMLProtocol, "viewer", identity); err == nil {
		t.Fatal("managed profile link was accepted")
	}
	if err := profiles.Link(SAMLProtocol, "missing", identity); err == nil {
		t.Fatal("missing profile link was accepted")
	}
	wantErr := errors.New("persist failed")
	values = []linkedProfile{{ID: "viewer"}}
	profiles, _ = linkedProfiles(&values, wantErr)
	if err := profiles.Link(OIDCProtocol, "viewer", identity); !errors.Is(err, wantErr) || values[0].OIDC != (Identity{}) {
		t.Fatalf("failed persistence = %#v %v", values, err)
	}
	profiles, _ = linkedProfiles(&values, nil)
	if err := profiles.Link(OIDCProtocol, "viewer", identity); err != nil {
		t.Fatal(err)
	}
	found, ok := profiles.Find(OIDCProtocol, identity)
	if !ok || found.ID != "viewer" {
		t.Fatalf("linked profile = %#v %v", found, ok)
	}
	if err := profiles.Unlink(OIDCProtocol, "viewer"); err != nil {
		t.Fatal(err)
	}
	if _, ok = profiles.Find(OIDCProtocol, identity); ok {
		t.Fatal("unlinked identity was found")
	}
}

func TestProfilesRejectInvalidLookupAndUnlinkWithoutEffects(t *testing.T) {
	t.Parallel()
	identity := Identity{Issuer: "issuer", Subject: "subject"}
	values := []linkedProfile{{ID: "viewer", OIDC: identity}}
	profiles, persisted := linkedProfiles(&values, nil)
	for _, protocol := range []Protocol{"", "LDAP"} {
		if _, ok := profiles.Find(protocol, identity); ok {
			t.Fatalf("invalid %q lookup matched a profile", protocol)
		}
		if err := profiles.Unlink(protocol, "viewer"); err == nil {
			t.Fatalf("invalid %q unlink was accepted", protocol)
		}
	}
	if _, ok := profiles.Find(OIDCProtocol, Identity{}); ok {
		t.Fatal("empty identity lookup matched a profile")
	}
	if *persisted != 0 || values[0].OIDC != identity {
		t.Fatalf("invalid operation caused effects: persisted=%d profile=%#v", *persisted, values[0])
	}
}

func TestProfilesAutoLinkSCIMRequiresOneActiveMatch(t *testing.T) {
	t.Parallel()
	identity := Identity{Issuer: "issuer", Subject: "directory"}
	for name, values := range map[string][]linkedProfile{
		"missing":   {{ID: "viewer", Managed: true, External: "other"}},
		"ambiguous": {{ID: "one", Managed: true, External: "directory"}, {ID: "two", Managed: true, External: "directory"}},
		"disabled":  {{ID: "viewer", Managed: true, External: "directory", Disabled: true}},
		"deleted":   {{ID: "viewer", Managed: true, External: "directory", Deleted: true}},
		"duplicate": {{ID: "viewer", Managed: true, External: "directory"}, {ID: "other", SAML: identity}},
	} {
		t.Run(name, func(t *testing.T) {
			profiles, persisted := linkedProfiles(&values, nil)
			if _, linked, err := profiles.AutoLinkSCIM(SAMLProtocol, identity); err != nil || linked || *persisted != 0 {
				t.Fatalf("invalid auto-link = %v %v persisted=%d", linked, err, *persisted)
			}
		})
	}
	values := []linkedProfile{{ID: "viewer", Managed: true, External: "directory"}}
	profiles, persisted := linkedProfiles(&values, nil)
	linked, ok, err := profiles.AutoLinkSCIM(SAMLProtocol, identity)
	if err != nil || !ok || linked.ID != "viewer" || values[0].SAML != identity || *persisted != 1 {
		t.Fatalf("valid auto-link = %#v %v %v persisted=%d", linked, ok, err, *persisted)
	}
}

func linkedProfiles(values *[]linkedProfile, persistErr error) (Profiles[linkedProfile], *int) {
	var mutex sync.Mutex
	persisted := new(int)
	return Profiles[linkedProfile]{
		Lock: mutex.Lock, Unlock: mutex.Unlock,
		Clone: func() []linkedProfile { return slices.Clone(*values) },
		Persist: func([]linkedProfile) error {
			*persisted++
			return persistErr
		},
		Commit: func(updated []linkedProfile) { *values = updated },
		Inspect: func(profile linkedProfile) Profile {
			return Profile{ID: profile.ID, SCIMExternalID: profile.External, OIDC: profile.OIDC, SAML: profile.SAML, SCIMManaged: profile.Managed, Disabled: profile.Disabled, SCIMDeleted: profile.Deleted}
		},
		Apply: func(profile *linkedProfile, protocol Protocol, identity Identity) {
			if protocol == SAMLProtocol {
				profile.SAML = identity
			} else {
				profile.OIDC = identity
			}
		},
	}, persisted
}

func TestLinkRejectsDisabledAndDeletedProfilesWithoutPersistence(t *testing.T) {
	for _, profile := range []linkedProfile{{ID: "viewer", Disabled: true}, {ID: "viewer", Deleted: true}} {
		for _, protocol := range []Protocol{OIDCProtocol, SAMLProtocol} {
			values := []linkedProfile{profile}
			profiles, persisted := linkedProfiles(&values, nil)
			if err := profiles.Link(protocol, "viewer", Identity{Issuer: "issuer", Subject: "subject"}); err == nil || *persisted != 0 || values[0].OIDC != (Identity{}) || values[0].SAML != (Identity{}) {
				t.Fatal("disabled or deleted profile gained a linked identity")
			}
		}
	}
}
