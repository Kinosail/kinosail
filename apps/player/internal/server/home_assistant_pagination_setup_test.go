package server_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func q12PaginationServer(t *testing.T, media string) (http.Handler, string) {
	t.Helper()
	data := t.TempDir()
	q12WriteLibrarySettings(t, data)
	handler := server.New(server.Config{
		Lifecycle: t.Context(), MediaDir: media, DataDir: data,
		CacheDir: t.TempDir(), RequireAuth: true, FFmpeg: "/q12-unavailable", FFprobe: "/q12-unavailable",
	})
	setup := apiCall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{
		"name": "Owner", "password": "owner-password", "device": "API test", "totp": true,
		"automaticUpdates": false,
	})
	assertAPIBody(t, setup, http.StatusCreated)
	var session struct {
		Token string `json:"token"`
		TOTP  struct {
			Secret string `json:"secret"`
		} `json:"totp"`
	}
	mustJSON(t, setup, &session)
	assertAPIBody(t, apiCall(t, handler, session.Token, http.MethodPut, "/api/v1/me/mfa",
		map[string]any{"code": testTOTP(t, session.TOTP.Secret, time.Now())}), http.StatusOK)
	return handler, session.Token
}
