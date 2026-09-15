package identitycore

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/federation"
	"github.com/MikeO7/kinosail/packages/passkeys"
	"github.com/MikeO7/kinosail/packages/scim"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestNewProfile(t *testing.T) { //nolint:cyclop // One constructor matrix proves all validation and dependency outcomes.
	t.Parallel()
	profile, err := NewProfile(" Owner ", "distinct-owner-password", true)
	if err != nil || profile.ID == "" || profile.Credential == "" || profile.Name != "Owner" || !profile.Owner || profile.Revision != 1 {
		t.Fatalf("NewProfile() = %#v, %v", profile, err)
	}
	for name, password := range map[string]string{"": "distinct-owner-password", strings.Repeat("x", 65): "distinct-owner-password", "Viewer": "short"} {
		if profile, err = NewProfile(name, password, false); err == nil || !reflect.DeepEqual(profile, Profile{}) {
			t.Fatalf("NewProfile(%q) accepted invalid input: %#v, %v", name, profile, err)
		}
	}

	wantErr := errors.New("hash failed")
	profile, err = newProfile("Viewer", "password", false, func(string) (string, error) {
		return "", wantErr
	}, func() string { return "unused" })
	if !errors.Is(err, wantErr) || !reflect.DeepEqual(profile, Profile{}) {
		t.Fatalf("newProfile(error) = %#v, %v", profile, err)
	}

	profile, err = newProfile("Viewer", "password", false, func(value string) (string, error) {
		return "hash:" + value, nil
	}, func() string { return "viewer-id" })
	want := Profile{ID: "viewer-id", Name: "Viewer", Credential: "hash:password", Revision: 1}
	if err != nil || !reflect.DeepEqual(profile, want) {
		t.Fatalf("newProfile(success) = %#v, %v; want %#v", profile, err, want)
	}
}

func TestCloneProfilesDetachesMutableState(t *testing.T) { //nolint:cyclop // Exact mutable-field assertions protect deep-copy semantics.
	t.Parallel()
	original := []Profile{{
		ID: "viewer", Libraries: []string{"movies"}, SCIMEmails: []scim.Email{{Value: "viewer@example.test"}},
		Recovery: []string{"recovery"}, Scopes: []string{"library"},
		Passkeys: []webauthn.Credential{{
			ID: []byte{1}, PublicKey: []byte{2}, Authenticator: webauthn.Authenticator{AAGUID: []byte{3}},
			Attestation: webauthn.CredentialAttestation{ClientDataJSON: []byte{4}, ClientDataHash: []byte{5}, AuthenticatorData: []byte{6}, Object: []byte{7}},
		}},
		PasskeyUsage: map[string]passkeys.Usage{"key": {LastUsed: 1}},
	}}
	clone := CloneProfiles(original)
	if !reflect.DeepEqual(clone, original) {
		t.Fatalf("CloneProfiles() = %#v; want %#v", clone, original)
	}
	clone[0].Libraries[0], clone[0].SCIMEmails[0].Value = "shows", "changed@example.test"
	clone[0].Recovery[0], clone[0].Scopes[0] = "changed", "admin"
	clone[0].Passkeys[0].ID[0], clone[0].Passkeys[0].PublicKey[0] = 9, 9
	clone[0].Passkeys[0].Authenticator.AAGUID[0], clone[0].Passkeys[0].Attestation.Object[0] = 9, 9
	clone[0].PasskeyUsage["key"] = passkeys.Usage{LastUsed: 9}
	if original[0].Libraries[0] != "movies" || original[0].SCIMEmails[0].Value != "viewer@example.test" ||
		original[0].Recovery[0] != "recovery" || original[0].Scopes[0] != "library" ||
		original[0].Passkeys[0].ID[0] != 1 || original[0].Passkeys[0].PublicKey[0] != 2 ||
		original[0].Passkeys[0].Authenticator.AAGUID[0] != 3 || original[0].Passkeys[0].Attestation.Object[0] != 7 ||
		original[0].PasskeyUsage["key"].LastUsed != 1 {
		t.Fatalf("CloneProfiles shared state with input: %#v", original)
	}
	if CloneProfiles(nil) != nil {
		t.Fatal("CloneProfiles(nil) did not preserve nil")
	}
}

func TestFindAndFederateProfile(t *testing.T) { //nolint:cyclop // One identity matrix covers lookup and both federation protocols.
	t.Parallel()
	profiles := []Profile{{ID: "viewer", Libraries: []string{"all"}}}
	found, ok := FindProfile(profiles, "viewer")
	if !ok || found.ID != "viewer" {
		t.Fatalf("FindProfile() = %#v, %v", found, ok)
	}
	found.Libraries[0] = "changed"
	if profiles[0].Libraries[0] != "all" {
		t.Fatal("FindProfile returned shared state")
	}
	if missing, ok := FindProfile(profiles, "missing"); ok || !reflect.DeepEqual(missing, Profile{}) {
		t.Fatalf("FindProfile(missing) = %#v, %v", missing, ok)
	}

	profile := Profile{ID: "viewer", SCIMExternalID: "external", OIDCIssuer: "oidc", OIDCSubject: "oidc-subject", SAMLIssuer: "saml", SAMLSubject: "saml-subject", SCIMManaged: true, Disabled: true, SCIMDeleted: true}
	want := federation.Profile{ID: "viewer", SCIMExternalID: "external", OIDC: federation.Identity{Issuer: "oidc", Subject: "oidc-subject"}, SAML: federation.Identity{Issuer: "saml", Subject: "saml-subject"}, SCIMManaged: true, Disabled: true, SCIMDeleted: true}
	if got := FederatedProfile(profile); !reflect.DeepEqual(got, want) {
		t.Fatalf("FederatedProfile() = %#v; want %#v", got, want)
	}
	ApplyFederatedIdentity(&profile, federation.SAMLProtocol, federation.Identity{Issuer: "new-saml", Subject: "new-saml-subject"})
	ApplyFederatedIdentity(&profile, federation.OIDCProtocol, federation.Identity{Issuer: "new-oidc", Subject: "new-oidc-subject"})
	if profile.SAMLIssuer != "new-saml" || profile.SAMLSubject != "new-saml-subject" || profile.OIDCIssuer != "new-oidc" || profile.OIDCSubject != "new-oidc-subject" {
		t.Fatalf("ApplyFederatedIdentity() = %#v", profile)
	}
}

func TestProfileFactorAndScopePolicy(t *testing.T) {
	t.Parallel()
	if (Profile{}).Secured() || !(Profile{TOTPSecret: "secret"}).Secured() || !(Profile{Passkeys: []webauthn.Credential{{}}}).Secured() {
		t.Fatal("Secured returned an unexpected result")
	}
	if !(Profile{}).Permits("library", true) || (Profile{APIKey: true, Scopes: []string{"admin"}}).Permits("library", true) || (Profile{APIKey: true, Scopes: []string{"library"}}).Permits("library", false) {
		t.Fatal("Permits returned an unexpected result")
	}
	routes := APIRoutes{Library: RouteSet("GET /library")}
	if !(Profile{Scopes: []string{"library"}}).AllowsAPI("GET /library", routes) || (Profile{Scopes: []string{"admin"}}).AllowsAPI("GET /library", routes) {
		t.Fatal("AllowsAPI returned an unexpected result")
	}
}
