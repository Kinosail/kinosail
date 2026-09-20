package identitycore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikeO7/kinosail/packages/scim"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestSCIMRestoreRequiresSameStableIdentity(t *testing.T) {
	for _, ids := range [][2]string{{"old", "new"}, {"", "new"}, {"old", ""}, {"", ""}} {
		t.Run(ids[0]+"/"+ids[1], func(t *testing.T) {
			fixture, store := newProfileStoreFixture(
				Profile{ID: "owner", Name: "Owner", Owner: true},
				Profile{ID: "deleted", Name: "Old", SCIMManaged: true, SCIMDeleted: true, SCIMUserName: "viewer@example.com", SCIMExternalID: ids[0]},
			)
			before := CloneProfiles(fixture.profiles)
			input := scimInput("Viewer", "VIEWER@example.com", true)
			input.ExternalID = ids[1]
			if _, err := store.CreateSCIMProfile(input); !errors.Is(err, scim.ErrConflict) {
				t.Fatalf("different or unknown identity restored: %v", err)
			}
			if len(fixture.writes) != 0 || !reflect.DeepEqual(before, fixture.profiles) {
				t.Fatal("rejected restore changed storage")
			}
		})
	}
}

func TestSCIMIdentityChangeRevokesFederationAndSessionsAtomically(t *testing.T) {
	for _, failFile := range []string{"profiles", "sessions", "keys", ""} {
		t.Run(failFile, func(t *testing.T) {
			fixture, store := newProfileStoreFixture(Profile{
				ID: "viewer", Name: "Viewer", SCIMManaged: true, SCIMUserName: "viewer@example.com", SCIMExternalID: "old",
				OIDCIssuer: "oidc", OIDCSubject: "old", SAMLIssuer: "saml", SAMLSubject: "old", Revision: 1,
				Credential: "password", TOTPSecret: "totp", Recovery: []string{"recovery"}, Passkeys: []webauthn.Credential{{ID: []byte("old")}},
			})
			fixture.sessions["viewer-session"] = Session{ProfileID: "viewer"}
			fixture.sessions["other-session"] = Session{ProfileID: "other"}
			fixture.keys["old-key"] = APIKey{ProfileID: "viewer"}
			fixture.keys["other-key"] = APIKey{ProfileID: "other"}
			before := CloneProfiles(fixture.profiles)
			fixture.failFile = failFile
			updated, err := store.UpdateSCIMProfile("viewer", scimInput("Viewer", "viewer@example.com", true), "")
			if failFile != "" {
				if err == nil || !reflect.DeepEqual(before, fixture.profiles) || len(fixture.sessions) != 2 || len(fixture.keys) != 2 {
					t.Fatalf("failed update changed state: %v", err)
				}
				return
			}
			if err != nil || updated.OIDCSubject != "" || updated.OIDCIssuer != "" || updated.SAMLSubject != "" || updated.SAMLIssuer != "" || updated.Revision != 2 {
				t.Fatalf("identity change retained federation: %#v %v", updated, err)
			}
			if len(fixture.sessions) != 1 || fixture.sessions["other-session"].ProfileID != "other" {
				t.Fatal("identity change did not revoke only affected sessions")
			}
			if updated.Credential != "" || updated.TOTPSecret != "" || len(updated.Recovery) != 0 || len(updated.Passkeys) != 0 || len(fixture.keys) != 1 || fixture.keys["other-key"].ProfileID != "other" {
				t.Fatal("previous identity retained a credential")
			}
		})
	}
}
