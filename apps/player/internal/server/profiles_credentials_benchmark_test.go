package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkTokenAccessDuringPasswordReset(b *testing.B) {
	store := newProfileStore(b.TempDir())
	owner, err := newProfile("Owner", "owner-password", true)
	if err != nil {
		b.Fatal(err)
	}
	viewer, err := newProfile("Viewer", "viewer-password", false)
	if err != nil {
		b.Fatal(err)
	}
	if err := store.save([]viewerProfile{owner, viewer}); err != nil {
		b.Fatal(err)
	}
	store.profiles = []viewerProfile{owner, viewer}
	ctx, stop := context.WithCancel(b.Context())
	resetting := make(chan struct{})
	go func() {
		defer close(resetting)
		for ctx.Err() == nil {
			if err := store.resetPassword(viewer.ID, "replacement-password"); err != nil {
				b.Error(err)
				return
			}
		}
	}()
	request := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer ks_missing")
	b.ResetTimer()
	for b.Loop() {
		store.access(request)
	}
	b.StopTimer()
	stop()
	<-resetting
}
