package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRejectedProgressCannotEmitPlaybackAudit(t *testing.T) {
	store := newProgressStore("")
	audit := newAuditStore(t.Context(), "", nil)
	store.SetAuditor(audit)
	request := httptest.NewRequestWithContext(withViewer(t.Context(), viewerProfile{ID: "viewer"}), http.MethodPut, "/", nil)
	accepted, err := store.SetRevision(request, "item", 120, nil, "phone", 2)
	if err != nil || !accepted || len(audit.Query("playback", 10)) != 1 {
		t.Fatalf("initial progress = %v %v", accepted, err)
	}
	watched := true
	accepted, err = store.SetRevision(request, "item", 10, &watched, "phone", 1)
	if err != nil || accepted || len(audit.Query("playback", 10)) != 1 || store.Get(request, "item").Seconds != 120 {
		t.Fatalf("stale progress changed state or audit: %v %v", accepted, err)
	}
}
