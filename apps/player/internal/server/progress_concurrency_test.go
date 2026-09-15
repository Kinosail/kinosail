package server

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProgressReadsRemainResponsiveAndCommittedWhileStateIsPersisted(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	store := newProgressStore(t.TempDir())
	store.SetPersistence(func(string, any) error {
		close(started)
		<-release
		return nil
	})
	request := httptest.NewRequestWithContext(t.Context(), "PUT", "/", nil)
	request = request.WithContext(context.WithValue(request.Context(), viewerContextKey{}, viewerProfile{ID: "viewer"}))
	done := make(chan error, 1)
	go func() {
		_, err := store.SetRevision(request, "movie", 42, nil, "session", 1)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("progress persistence did not start")
	}
	read := make(chan playbackState, 1)
	go func() { read <- store.Get(request, "movie") }()
	select {
	case state := <-read:
		if state.Seconds != 0 {
			t.Fatalf("progress = %#v", state)
		}
	case <-time.After(25 * time.Millisecond):
		close(release)
		t.Fatal("progress read waited for filesystem persistence")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if state := store.Get(request, "movie"); state.Seconds != 42 {
		t.Fatalf("committed progress = %#v", state)
	}
}
