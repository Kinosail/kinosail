package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteSecurityMutationsHaveStableAuditActions(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ method, path, action string }{
		"share claim": {http.MethodPost, "/api/v1/media-shares/claim", "media-share.claimed"},
		"web kill":    {http.MethodPost, "/settings/remote/kill", "remote-access.killed"},
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
