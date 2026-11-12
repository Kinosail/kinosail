package identitycore

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type accessFlushFixture struct {
	lock     sync.RWMutex
	sessions map[string]Session
	keys     map[string]APIKey
	use      AccessUse
	state    AccessPersistence
	written  map[string]any
	writes   int
	locked   int
	fail     bool
}

func newAccessFlushFixture(now int64) *accessFlushFixture {
	return &accessFlushFixture{
		sessions: map[string]Session{"session": {ProfileID: "viewer", Browser: true, LastSeen: now - 120}},
		keys:     map[string]APIKey{"key": {ProfileID: "viewer", LastUsed: now - 7200}},
		written:  map[string]any{},
	}
}

func (fixture *accessFlushFixture) flusher() AccessFlusher {
	return AccessFlusher{
		Lock: &fixture.lock, Sessions: &fixture.sessions, APIKeys: &fixture.keys, Use: &fixture.use, State: &fixture.state,
		SessionFile: "sessions.json", APIFile: "api_keys.json",
		Persist: func(file string, value any) error {
			if fixture.lock.TryRLock() {
				fixture.lock.RUnlock()
			} else {
				fixture.locked++
			}
			fixture.writes++
			if fixture.fail {
				return errors.New("forced write failure")
			}
			fixture.written[file] = value
			return nil
		},
		RequestID: func(context.Context) string { return "request-1" },
	}
}

func TestAccessFlusherWritesEachUsedDocumentWithItsOwnState(t *testing.T) {
	now := time.Now().Unix()
	fixture := newAccessFlushFixture(now)
	flusher := fixture.flusher()
	flusher.Flush(t.Context())
	if fixture.writes != 0 {
		t.Fatalf("flush without use wrote %d documents", fixture.writes)
	}
	fixture.use.Record("session", fixture.sessions["session"], true, APIKey{}, false, now)
	flusher.Flush(t.Context())
	sessions, wroteSessions := fixture.written["sessions.json"].(map[string]Session)
	if fixture.writes != 1 || !wroteSessions || sessions["session"].LastSeen != now || fixture.written["api_keys.json"] != nil {
		t.Fatalf("session use flush wrote %d documents: %#v", fixture.writes, fixture.written)
	}
	fixture.use.Record("key", Session{}, false, fixture.keys["key"], true, now)
	flusher.Flush(t.Context())
	keys, wroteKeys := fixture.written["api_keys.json"].(map[string]APIKey)
	if fixture.writes != 2 || fixture.locked != 0 || !wroteKeys || keys["key"].LastUsed != now {
		t.Fatalf("API key use flush wrote %d documents: %#v", fixture.writes, fixture.written)
	}
}

func TestAccessFlusherRetriesAFailedWriteOnALaterFlush(t *testing.T) {
	now := time.Now().Unix()
	fixture := newAccessFlushFixture(now)
	flusher := fixture.flusher()
	fixture.fail = true
	fixture.lock.Lock()
	flusher.Save(t.Context(), "session-revoke", "sessions.json", now)
	fixture.lock.Unlock()
	if fixture.writes != 1 || fixture.state.Pending() != 1 {
		t.Fatalf("failed save: writes=%d pending=%d", fixture.writes, fixture.state.Pending())
	}
	fixture.locked = 0
	flusher.Flush(t.Context())
	if fixture.writes != 1 {
		t.Fatalf("flush retried before the retry time: writes=%d", fixture.writes)
	}
	fixture.fail = false
	fixture.state.ExpireRetries(time.Now().Unix())
	flusher.Flush(t.Context())
	if fixture.writes != 2 || fixture.locked != 0 || fixture.state.Pending() != 0 || fixture.written["sessions.json"] == nil {
		t.Fatalf("retry: writes=%d pending=%d written=%#v", fixture.writes, fixture.state.Pending(), fixture.written)
	}
}

func TestAccessFlusherFlushesUseWhenTheLifecycleEnds(t *testing.T) {
	now := time.Now().Unix()
	fixture := newAccessFlushFixture(now)
	flusher := fixture.flusher()
	lifecycle, end := context.WithCancel(t.Context())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		flusher.Run(lifecycle, time.Hour)
	}()
	fixture.use.Record("session", fixture.sessions["session"], true, APIKey{}, false, now)
	end()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("flusher did not stop with its lifecycle")
	}
	if sessions, ok := fixture.written["sessions.json"].(map[string]Session); !ok || sessions["session"].LastSeen != now {
		t.Fatalf("lifecycle end did not flush session use: %#v", fixture.written)
	}
}
