package servertest

import (
	"net/http"
	"testing"
	"time"
)

func (fixture APIParityFixture) OwnerCanCreateAnotherOwnerThroughAPI(t *testing.T) {
	t.Parallel()
	handler, token := fixture.Server(t)
	created := APICall(t, handler, token, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Partner", "password": "partner-password", "owner": true})
	AssertAPIBody(t, created, http.StatusCreated, `"name":"Partner"`)
	login := APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Partner", "password": "partner-password", "device": "Partner browser"})
	var partner struct {
		Token string `json:"token"`
	}
	MustJSON(t, login, &partner)
	AssertAPIBody(t, APICall(t, handler, partner.Token, http.MethodGet, "/api/v1/me", nil), http.StatusOK, `"owner":true`)
	AssertAPIBody(t, APICall(t, handler, partner.Token, http.MethodGet, "/api/v1/settings", nil), http.StatusForbidden, `"mfaEnrollmentRequired":true`)
	enrollment := APICall(t, handler, partner.Token, http.MethodPost, "/api/v1/me/mfa/setup", map[string]any{})
	var factor struct{ Secret string }
	MustJSON(t, enrollment, &factor)
	AssertAPIBody(t, APICall(t, handler, partner.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": fixture.TOTP(t, factor.Secret, time.Now())}), http.StatusOK, `"enabled":true`)
	if settings := APICall(t, handler, partner.Token, http.MethodGet, "/api/v1/settings", nil); settings.Code != http.StatusOK {
		t.Fatalf("second Owner settings = %d %q", settings.Code, settings.Body.String())
	}
}

func (fixture APIParityFixture) OwnerCanChangeProfileTypeThroughAPI(t *testing.T) {
	t.Parallel()
	handler, token := fixture.Server(t)
	created := APICall(t, handler, token, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Partner", "password": "partner-password"})
	var profile struct{ ID string }
	MustJSON(t, created, &profile)
	login := APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Partner", "password": "partner-password", "device": "Partner browser"})
	var partner struct {
		Token string `json:"token"`
	}
	MustJSON(t, login, &partner)
	AssertAPICalls(t, handler, token, []APITestCall{
		{Method: http.MethodPut, Path: "/api/v1/profiles/" + profile.ID, Body: map[string]any{"owner": true}, Status: http.StatusOK},
	})
	AssertAPIBody(t, APICall(t, handler, partner.Token, http.MethodGet, "/api/v1/settings", nil), http.StatusForbidden, `"mfaEnrollmentRequired":true`)
	enrollment := APICall(t, handler, partner.Token, http.MethodPost, "/api/v1/me/mfa/setup", map[string]any{})
	var factor struct{ Secret string }
	MustJSON(t, enrollment, &factor)
	AssertAPIBody(t, APICall(t, handler, partner.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": fixture.TOTP(t, factor.Secret, time.Now())}), http.StatusOK, `"enabled":true`)
	if settings := APICall(t, handler, partner.Token, http.MethodGet, "/api/v1/settings", nil); settings.Code != http.StatusOK {
		t.Fatalf("promoted Owner settings = %d %q", settings.Code, settings.Body.String())
	}
	AssertAPICalls(t, handler, token, []APITestCall{
		{Method: http.MethodPut, Path: "/api/v1/profiles/" + profile.ID, Body: map[string]any{"owner": false}, Status: http.StatusOK},
	})
	if settings := APICall(t, handler, partner.Token, http.MethodGet, "/api/v1/settings", nil); settings.Code != http.StatusForbidden {
		t.Fatalf("demoted Viewer settings = %d %q", settings.Code, settings.Body.String())
	}
}
