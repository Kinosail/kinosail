package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteSecurityMutationsHaveStableAuditActions(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ method, path, action string }{
		"kill":         {http.MethodPost, "/api/v1/remote-access/kill", "remote-access.killed"},
		"reset":        {http.MethodDelete, "/api/v1/remote-access/kill", "remote-access.reset"},
		"share create": {http.MethodPost, "/api/v1/media-shares", "media-share.created"},
		"share claim":  {http.MethodPost, "/api/v1/media-shares/claim", "media-share.claimed"},
		"share revoke": {http.MethodDelete, "/api/v1/media-shares/id", "media-share.revoked"},
		"web kill":     {http.MethodPost, "/settings/remote/kill", "remote-access.killed"},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), test.method, test.path, nil)
			action, _ := auditAction(request)
			if action != test.action {
				t.Fatalf("action = %q, want %q", action, test.action)
			}
		})
	}
}
