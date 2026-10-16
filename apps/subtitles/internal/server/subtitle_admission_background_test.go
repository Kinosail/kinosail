package server

import (
	"errors"
	"testing"
)

func TestBackgroundSubtitleWorkYieldsToWaitingOperation(t *testing.T) {
	t.Parallel()
	gate := &subtitleAdmission{}
	_, first, err := gate.enterBackground(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if gate.claim() {
		t.Fatal("operation claimed the gate while background work was running")
	}
	if _, _, err = gate.enterBackground(t.Context()); !errors.Is(err, errSubtitleBusy) {
		t.Fatalf("background work started while an operation was waiting: %v", err)
	}
	first()
	if !gate.claim() {
		t.Fatal("waiting operation could not claim the gate after background work stopped")
	}
	if _, _, err = gate.enterBackground(t.Context()); !errors.Is(err, errSubtitleBusy) {
		t.Fatalf("background work started during an operation: %v", err)
	}
	gate.release()
	_, settled, err := gate.enterBackground(t.Context())
	if err != nil {
		t.Fatalf("background work did not resume after the operation: %v", err)
	}
	settled()
	_, legacy, err := gate.enter(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if gate.claim() {
		t.Fatal("operation claimed the gate during a direct request")
	}
	legacy()
	if _, settled, err = gate.enterBackground(t.Context()); err != nil {
		t.Fatalf("a direct request made background work yield: %v", err)
	}
	settled()
}
