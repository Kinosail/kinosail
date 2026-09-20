package federation

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLinkCallbacksRejectRevokedInitiatingSession(t *testing.T) {
	for _, protocol := range []Protocol{OIDCProtocol, SAMLProtocol} {
		t.Run(string(protocol), func(t *testing.T) {
			values := []webProfile{{ID: "viewer"}}
			profiles := webProfiles(&values)
			active := true
			profiles.SessionActive = func(key, id string) bool { return active && key == "original" && id == "viewer" }
			effects := &webEffects{}
			identity := Identity{Issuer: "issuer", Subject: "subject"}
			active = false
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/callback", nil)
			if protocol == OIDCProtocol {
				login := NewOIDCHTTP(OIDCConfig{}, profiles, webHooks(effects), nil)
				login.acceptCallback(httptest.NewRecorder(), request, OIDCCallback{ProfileID: "viewer", LinkSession: "original", Identity: identity})
			} else {
				login := NewSAMLHTTP(SAMLConfig{}, profiles, webHooks(effects))
				login.acceptCallback(httptest.NewRecorder(), request, SAMLCallback{ProfileID: "viewer", LinkSession: "original", Identity: identity})
			}
			if values[0].OIDC != (Identity{}) || values[0].SAML != (Identity{}) || effects.errors != 1 {
				t.Fatal("revoked session persisted an identity link")
			}
			active = true
			if err := profiles.LinkForSession(protocol, "viewer", identity, "other-session"); err == nil {
				t.Fatal("unbound session linked an identity")
			}
			if err := profiles.LinkForSession(protocol, "viewer", identity, "original"); err != nil {
				t.Fatalf("active original session rejected: %v", err)
			}
		})
	}
}
