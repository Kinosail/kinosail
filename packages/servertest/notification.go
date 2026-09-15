package servertest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// WebhookReceivesRedactedAdministrativeEvents runs the corresponding app regression contract.
func WebhookReceivesRedactedAdministrativeEvents(t *testing.T, newHandler func(string, string, string) http.Handler, createProfile func(*testing.T, http.Handler)) {
	t.Parallel()
	events := make(chan string, 1)
	webhook := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		events <- string(body) + request.Header.Get("Authorization")
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer webhook.Close()
	handler := newHandler(t.TempDir(), webhook.URL, "notification-secret")
	createProfile(t, handler)
	for {
		select {
		case event := <-events:
			if strings.Contains(event, "profile.created") {
				if !strings.Contains(event, `"target":"Sam"`) || strings.Contains(event, "owner-password") || strings.Contains(event, "viewer-password") {
					t.Fatalf("unsafe event: %s", event)
				}
				return
			}
		case <-time.After(2 * time.Second):
			t.Fatal("profile event was not delivered")
		}
	}
}
