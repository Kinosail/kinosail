package servertest

import (
	"net/http"
	"testing"
	"time"
)

func aPIRejectsNegativeProgressAndBootstrapsTheViewer(t *testing.T, fixture APIParityFixture) {
	t.Parallel()

	handler, token := fixture.Server(t)
	var catalog struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	MustJSON(t, APICall(t, handler, token, http.MethodGet, "/api/v1/library", nil), &catalog)
	invalid := APICall(t, handler, token, http.MethodPut, "/api/v1/items/"+catalog.Items[0].ID+"/progress", map[string]any{"seconds": -1})
	AssertAPIBody(t, invalid, http.StatusBadRequest, `"error"`)
	me := APICall(t, handler, token, http.MethodGet, "/api/v1/me", nil)
	AssertAPIBody(t, me, http.StatusOK, `"server":"Kinosail"`, `"name":"Owner"`, `"owner":true`, `"downloads":true`, `"transcode":true`, `"remote":true`)
}

func playbackAPIReturnsAutomaticNextEpisode(t *testing.T, fixture APIParityFixture) {
	t.Parallel()

	handler, token := fixture.Server(t)
	var shows struct {
		Shows []struct {
			ID string `json:"id"`
		} `json:"shows"`
	}
	MustJSON(t, APICall(t, handler, token, http.MethodGet, "/api/v1/shows", nil), &shows)
	var show struct {
		Episodes []struct {
			ID string `json:"id"`
		} `json:"episodes"`
	}
	MustJSON(t, APICall(t, handler, token, http.MethodGet, "/api/v1/shows/"+shows.Shows[0].ID, nil), &show)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/items/"+show.Episodes[0].ID+"/playback", nil), http.StatusOK, `"autoSkip":[]`, `"next":"`+show.Episodes[1].ID+`"`)
}

func ownerCanBootstrapThroughAPI(t *testing.T, fixture APIParityFixture) {
	t.Parallel()

	handler := fixture.Bootstrap(t)
	setup := APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "device": "Installer", "totp": true})
	var session struct {
		Token string                  `json:"token"`
		TOTP  struct{ Secret string } `json:"totp"`
	}
	MustJSON(t, setup, &session)
	if setup.Code != http.StatusCreated || session.Token == "" {
		t.Fatalf("setup = %d %q", setup.Code, setup.Body.String())
	}
	AssertAPIBody(t, APICall(t, handler, session.Token, http.MethodGet, "/api/v1/settings", nil), http.StatusForbidden, `"mfaEnrollmentRequired":true`)
	AssertAPIBody(t, APICall(t, handler, session.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": fixture.TOTP(t, session.TOTP.Secret, time.Now())}), http.StatusOK, `"enabled":true`)
	AssertAPIBody(t, APICall(t, handler, session.Token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"name":"Kinosail"`)
	conflict := APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Other", "password": "other-password"})
	if conflict.Code != http.StatusConflict {
		t.Fatalf("repeated setup = %d %q", conflict.Code, conflict.Body.String())
	}
}
