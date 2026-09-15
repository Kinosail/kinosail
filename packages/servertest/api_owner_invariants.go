package servertest

import (
	"net/http"
	"strings"
	"testing"
)

func (fixture APIParityFixture) ServerKeepsAtLeastOneOwner(t *testing.T) {
	t.Parallel()
	handler, token := fixture.Server(t)
	var result struct {
		Profiles []struct {
			ID    string `json:"id"`
			Owner bool   `json:"owner"`
		} `json:"profiles"`
	}
	MustJSON(t, APICall(t, handler, token, http.MethodGet, "/api/v1/profiles", nil), &result)
	if len(result.Profiles) != 1 || !result.Profiles[0].Owner {
		t.Fatalf("initial profiles = %+v", result.Profiles)
	}
	response := APICall(t, handler, token, http.MethodPut, "/api/v1/profiles/"+result.Profiles[0].ID, map[string]any{"owner": false})
	AssertAPIBody(t, response, http.StatusBadRequest, `"error":"at least one Owner Profile is required"`)
	if settings := APICall(t, handler, token, http.MethodGet, "/api/v1/settings", nil); settings.Code != http.StatusOK {
		t.Fatalf("last Owner settings = %d %q", settings.Code, settings.Body.String())
	}
}

func (fixture APIParityFixture) OwnerCanRemoveAnotherOwnerButNotTheLast(t *testing.T) {
	t.Parallel()
	handler, token := fixture.Server(t)
	created := APICall(t, handler, token, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Partner", "password": "partner-password", "owner": true})
	var partner struct{ ID string }
	MustJSON(t, created, &partner)
	login := APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Partner", "password": "partner-password", "device": "Partner browser"})
	var session struct {
		Token string `json:"token"`
	}
	MustJSON(t, login, &session)
	key := APICall(t, handler, session.Token, http.MethodPost, "/api/v1/api-keys", map[string]any{"name": "Partner automation", "scopes": "admin"})
	var credential struct {
		Secret string `json:"secret"`
	}
	MustJSON(t, key, &credential)
	if removed := APICall(t, handler, token, http.MethodDelete, "/api/v1/profiles/"+partner.ID, nil); removed.Code != http.StatusNoContent {
		t.Fatalf("remove second Owner = %d %q", removed.Code, removed.Body.String())
	}
	if me := APICall(t, handler, session.Token, http.MethodGet, "/api/v1/me", nil); me.Code != http.StatusUnauthorized {
		t.Fatalf("removed Owner session = %d %q", me.Code, me.Body.String())
	}
	if me := APICall(t, handler, credential.Secret, http.MethodGet, "/api/v1/me", nil); me.Code != http.StatusUnauthorized {
		t.Fatalf("removed Owner API key = %d %q", me.Code, me.Body.String())
	}
	if keys := APICall(t, handler, token, http.MethodGet, "/api/v1/api-keys", nil); strings.Contains(keys.Body.String(), "Partner automation") {
		t.Fatalf("removed Owner API key remains = %q", keys.Body.String())
	}
	var result struct {
		Profiles []struct{ ID string } `json:"profiles"`
	}
	MustJSON(t, APICall(t, handler, token, http.MethodGet, "/api/v1/profiles", nil), &result)
	if removed := APICall(t, handler, token, http.MethodDelete, "/api/v1/profiles/"+result.Profiles[0].ID, nil); removed.Code != http.StatusBadRequest {
		t.Fatalf("remove last Owner = %d %q", removed.Code, removed.Body.String())
	}
}
