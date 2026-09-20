package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/casting"
)

func TestCastCommandsRejectInvalidOwnershipPositionAndOfflineReceiver(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, body, viewer, protocol string
		status                       int
	}{
		{"malformed", "{", "local-owner", "dlna", 400},
		{"unknown action", `{"action":"unknown"}`, "local-owner", "dlna", 400},
		{"other viewer", `{"action":"play"}`, "other", "dlna", 404},
		{"wrong protocol", `{"action":"play"}`, "local-owner", "airplay", 404},
		{"outside duration", `{"action":"seek","position":101}`, "local-owner", "dlna", 400},
		{"offline receiver", `{"action":"play"}`, "local-owner", "dlna", 503},
	} {
		t.Run(test.name, func(t *testing.T) {
			id := strings.Repeat("a", 32)
			session := castSession{ID: id, profileID: "local-owner", Protocol: test.protocol, Duration: 100, DeviceID: strings.Repeat("b", 32), ExpiresAt: time.Now().Add(time.Hour)}
			service := &castService{sessions: map[string]castSession{id: session}, auth: &authentication{profiles: newProfileStore("")}, renderers: casting.NewRenderers(), now: time.Now}
			request := httptest.NewRequestWithContext(withViewer(t.Context(), viewerProfile{ID: test.viewer, Owner: true}), "POST", "/", strings.NewReader(test.body))
			request.SetPathValue("id", id)
			response := httptest.NewRecorder()
			service.commandHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.status, response.Body)
			}
			if _, found := service.session(id); !found {
				t.Fatal("rejected command revoked session")
			}
		})
	}
}
