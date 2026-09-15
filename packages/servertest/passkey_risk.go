package servertest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/auditjournal"
	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/go-webauthn/webauthn/webauthn"
)

// PasskeyCloneWarningCreatesSecuritySignal verifies clone warnings remain advisory and auditable.
func PasskeyCloneWarningCreatesSecuritySignal(t *testing.T, risk func(*http.Request, identitycore.Profile, *webauthn.Credential), query func(string, int) []auditjournal.Event) {
	t.Parallel()
	profile := identitycore.Profile{ID: "viewer", Name: "Viewer"}
	credential := &webauthn.Credential{ID: []byte("credential"), Authenticator: webauthn.Authenticator{SignCount: 4, CloneWarning: true}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/passkeys/login/finish", nil)
	risk(request, profile, credential)
	events := query("", 10)
	if len(events) != 1 || events[0].Action != "passkey.risk" || events[0].Result != "warning" || events[0].ActorID != profile.ID {
		t.Fatalf("risk event = %#v", events)
	}
}
