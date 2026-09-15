package server

import (
	"net/http"
	"testing"
)

func observedAuditHandler(t *testing.T, pattern string, handler http.Handler) http.Handler {
	t.Helper()
	return observeRequests(newAuditStore(t.Context(), t.TempDir(), nil), func(*http.Request) string { return pattern }, handler)
}

func setPrivateAuditViewer(request *http.Request) {
	setAuditViewer(request, viewerProfile{ID: "private-profile-id", Name: "Private Viewer"})
}
