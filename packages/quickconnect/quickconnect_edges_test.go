package quickconnect

import (
	"errors"
	"testing"
	"time"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("random source failed") }

func TestBrokerCoversDefaultExpiryAndMissingApproval(t *testing.T) {
	broker := New(0)
	if broker.ttl != defaultTTL {
		t.Fatalf("default TTL = %v", broker.ttl)
	}
	if err := broker.Approve("missing", Viewer{ID: "viewer"}); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("missing approval error = %v", err)
	}
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	broker.now = func() time.Time { return now }
	_, connection, err := broker.Create(Request{Device: "TV"})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(defaultTTL)
	if err := broker.Approve(connection.Code, Viewer{ID: "viewer"}); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("expired approval error = %v", err)
	}
	var nilBroker *Broker
	nilBroker.RevokeRemote()
}

func TestCreateRetriesCollisionsAndReportsCodeFailure(t *testing.T) {
	broker := New(time.Minute)
	broker.byCode["COLLIDE"] = "existing"
	calls := 0
	broker.code = func(bool) (string, error) {
		calls++
		if calls == 1 {
			return "COLLIDE", nil
		}
		return "UNIQUE", nil
	}
	if _, connection, err := broker.Create(Request{}); err != nil || connection.Code != "UNIQUE" {
		t.Fatalf("Create() = %#v, %v", connection, err)
	}
	broker.code = func(bool) (string, error) { return "", errors.New("code generation failed") }
	if _, _, err := broker.Create(Request{}); err == nil {
		t.Fatal("code generation failure was ignored")
	}
}

func TestNumericCodeReportsRandomSourceFailure(t *testing.T) {
	if code, err := connectCode(true, failingReader{}); err == nil || code != "" {
		t.Fatalf("connectCode() = %q, %v", code, err)
	}
}
