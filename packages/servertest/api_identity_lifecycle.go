package servertest

import (
	"net/http"
	"testing"
)

func (fixture APIParityFixture) OwnerIdentityLifecycleAPIRoutes(t *testing.T) {
	t.Parallel()
	handler, token := fixture.Server(t)
	created := APICall(t, handler, token, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Guest", "password": "guest-password", "rating": "teen"})
	var profile struct{ ID string }
	MustJSON(t, created, &profile)
	AssertAPICalls(t, handler, token, []APITestCall{
		{Method: http.MethodPut, Path: "/api/v1/profiles/" + profile.ID, Body: map[string]any{"rating": "family", "downloads": true, "libraries": []string{"."}}, Status: http.StatusOK},
		{Method: http.MethodPut, Path: "/api/v1/profiles/" + profile.ID + "/password", Body: map[string]any{"password": "new-guest-password"}, Status: http.StatusNoContent},
	})
	login := APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Guest", "password": "new-guest-password", "device": "Guest browser"})
	var guest struct {
		Token string `json:"token"`
	}
	MustJSON(t, login, &guest)
	AssertAPIBody(t, APICall(t, handler, guest.Token, http.MethodGet, "/api/v1/me", nil), http.StatusOK, `"server":"Kinosail"`, `"name":"Guest"`, `"owner":false`, `"downloads":true`, `"transcode":false`, `"remote":false`, `"rating":"family"`)
	if response := APICall(t, handler, guest.Token, http.MethodGet, "/api/v1/settings", nil); response.Code != http.StatusForbidden {
		t.Fatalf("viewer settings = %d %q", response.Code, response.Body.String())
	}
	key := APICall(t, handler, token, http.MethodPost, "/api/v1/api-keys", map[string]any{"name": "Temporary", "scopes": "library"})
	AssertAPIBody(t, key, http.StatusCreated, `"secret":"ks_`)
	var scoped struct {
		Secret string `json:"secret"`
	}
	MustJSON(t, key, &scoped)
	AssertAPIBody(t, APICall(t, handler, scoped.Secret, http.MethodGet, "/api/v1/me", nil), http.StatusOK, `"owner":false`, `"downloads":false`, `"transcode":false`, `"remote":true`)
	admin := APICall(t, handler, token, http.MethodPost, "/api/v1/api-keys", map[string]any{"name": "Admin", "scopes": "admin"})
	MustJSON(t, admin, &scoped)
	AssertAPIBody(t, APICall(t, handler, scoped.Secret, http.MethodGet, "/api/v1/me", nil), http.StatusOK, `"owner":true`, `"downloads":false`, `"transcode":false`, `"remote":false`)
	var keys struct {
		Keys []struct {
			ID string `json:"id"`
		} `json:"keys"`
	}
	MustJSON(t, APICall(t, handler, token, http.MethodGet, "/api/v1/api-keys", nil), &keys)
	AssertAPICalls(t, handler, token, []APITestCall{
		{Method: http.MethodDelete, Path: "/api/v1/api-keys/" + keys.Keys[0].ID, Body: nil, Status: http.StatusNoContent},
		{Method: http.MethodDelete, Path: "/api/v1/devices/missing", Body: nil, Status: http.StatusBadRequest},
		{Method: http.MethodDelete, Path: "/api/v1/sessions", Body: nil, Status: http.StatusNoContent},
		{Method: http.MethodDelete, Path: "/api/v1/profiles/" + profile.ID, Body: nil, Status: http.StatusNoContent},
	})
}
