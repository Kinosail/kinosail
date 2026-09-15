package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestWebhookReceivesRedactedAdministrativeEvents(t *testing.T) {
	servertest.WebhookReceivesRedactedAdministrativeEvents(t, func(dataDir, url, token string) http.Handler {
		return server.New(server.Config{DataDir: dataDir, RequireAuth: true, Notifications: server.NotificationConfig{URL: url, Token: token}})
	}, func(t *testing.T, handler http.Handler) {
		owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
		requestWithCookie(t, handler, http.MethodPost, "/settings/profiles", "name=Sam&password=viewer-password", owner)
	})
}
