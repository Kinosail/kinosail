package identitycore

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/scim"
)

func validManagedProfile(id, userName string) Profile {
	now := time.Date(2026, time.September, 4, 18, 0, 0, 0, time.UTC)
	return Profile{
		ID: id, Name: "Viewer " + id, SCIMManaged: true, SCIMUserName: userName,
		SCIMCreatedAt: now, SCIMUpdatedAt: now,
	}
}

func TestValidateProfiles(t *testing.T) {
	if err := ValidateProfiles(nil); err != nil {
		t.Fatal(err)
	}
	local := Profile{ID: "local", Name: "Local"}
	managed := validManagedProfile("managed", "viewer@example.com")
	managed.SCIMName = scim.Name{Formatted: "Viewer"}
	managed.SCIMEmails = []scim.Email{{Value: "viewer@example.com", Type: "work", Primary: true}}
	managed.SCIMEnterprise = scim.EnterpriseProfile{Department: "Media"}
	linked := local
	linked.ID = "linked"
	linked.OIDCIssuer, linked.OIDCSubject = "https://identity.example", "oidc-subject"
	linked.SAMLIssuer, linked.SAMLSubject = "https://identity.example/saml", "saml-subject"
	if err := ValidateProfiles([]Profile{local, managed, linked}); err != nil {
		t.Fatalf("valid profiles: %v", err)
	}

	invalid := map[string][]Profile{
		"duplicate ID":         {local, local},
		"missing ID":           {{Name: "Missing"}},
		"duplicate userName":   {managed, validManagedProfile("other", "viewer@example.com")},
		"unmanaged SCIM state": {{ID: "bad", SCIMExternalID: "unexpected"}},
		"managed credentials":  {func() Profile { profile := managed; profile.Credential = "hash"; return profile }()},
		"managed identity": {func() Profile {
			profile := managed
			profile.SCIMUpdatedAt = profile.SCIMCreatedAt.Add(-time.Second)
			return profile
		}()},
		"invalid SCIM input": {func() Profile { profile := managed; profile.Name = ""; return profile }()},
		"changed normalization": {func() Profile {
			profile := managed
			profile.Name = " Viewer "
			return profile
		}()},
		"incomplete OIDC": {{ID: "bad", OIDCIssuer: "https://identity.example"}},
		"incomplete SAML": {{ID: "bad", SAMLSubject: "subject"}},
		"invalid issuer":  {{ID: "bad", OIDCIssuer: " bad ", OIDCSubject: "subject"}},
		"invalid subject": {{ID: "bad", OIDCIssuer: "https://identity.example", OIDCSubject: "bad\x00subject"}},
		"duplicate OIDC": {linked, {
			ID: "duplicate", OIDCIssuer: linked.OIDCIssuer, OIDCSubject: linked.OIDCSubject,
		}},
		"duplicate SAML": {linked, {
			ID: "duplicate", SAMLIssuer: linked.SAMLIssuer, SAMLSubject: linked.SAMLSubject,
		}},
	}
	for name, profiles := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := ValidateProfiles(profiles); err == nil {
				t.Fatal("invalid profiles were accepted")
			}
		})
	}
}

func TestPersistedSCIMComparisonEdges(t *testing.T) {
	profile := validManagedProfile("managed", "viewer@example.com")
	input := scimInput(profile.Name, profile.SCIMUserName, true)
	profile.SCIMName = scim.Name{Formatted: input.Formatted, GivenName: input.GivenName, FamilyName: input.FamilyName}
	profile.SCIMExternalID, profile.SCIMEmails, profile.SCIMEnterprise = input.ExternalID, input.Emails, input.Enterprise
	if !samePersistedSCIMProfile(input, profile) {
		t.Fatal("equal persisted input did not match")
	}
	changed := input
	changed.Emails = append(changed.Emails, scim.Email{Value: "extra@example.com"})
	if samePersistedSCIMProfile(changed, profile) {
		t.Fatal("different email length matched")
	}
	changed = input
	changed.Emails = append([]scim.Email(nil), input.Emails...)
	changed.Emails[0].Type = "home"
	if samePersistedSCIMProfile(changed, profile) {
		t.Fatal("different email matched")
	}
	profile.Owner = true
	if validManagedSCIMProfile(profile) {
		t.Fatal("owner accepted as managed profile")
	}
	profile.Owner, profile.SCIMDeleted, profile.Disabled = false, true, false
	if validManagedSCIMProfile(profile) {
		t.Fatal("active deleted profile was accepted")
	}
	if validUnmanagedSCIMProfile(profile) {
		t.Fatal("managed state accepted as unmanaged")
	}
}

func TestValidIdentityStringEdges(t *testing.T) {
	if value, err := validIdentityString("identity", 8); err != nil || value != "identity" {
		t.Fatalf("valid identity = %q, %v", value, err)
	}
	if value, err := validIdentityString(" identity ", 16); err != nil || value != "identity" {
		t.Fatalf("trimmed identity = %q, %v", value, err)
	}
	for name, value := range map[string]string{
		"oversized": strings.Repeat("x", 9),
		"control":   "bad\nvalue",
		"utf8":      string([]byte{utf8.RuneSelf}),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := validIdentityString(value, 8); err == nil {
				t.Fatal("invalid identity was accepted")
			}
		})
	}
}
