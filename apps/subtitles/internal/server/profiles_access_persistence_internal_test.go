package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAccessStateWriteFailuresAreLoggedRetriedAndThrottled(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	now := time.Now().Unix()
	failing := true
	attempts, persisted := map[string]int{}, map[string]string{}
	store := &profileStore{
		file: "profiles.json", sessionFile: "sessions.json", apiFile: "api_keys.json",
		profiles: []viewerProfile{{ID: "viewer", Revision: 2}},
		sessions: map[string]viewerSession{
			sessionKey("expired-token"):      {ProfileID: "viewer", Browser: true, CreatedAt: now - 7200, LastSeen: now - 3600, ExpiresAt: now + 3600},
			sessionKey("stale-public-token"): {ProfileID: "viewer", Channel: "public", ProfileRevision: 1, CreatedAt: now, ExpiresAt: now + 3600},
			sessionKey("active-token"):       {ProfileID: "viewer", Browser: true, CreatedAt: now, LastSeen: now, ExpiresAt: now + 3600},
		},
		apiKeys: map[string]apiKey{sessionKey("expired-key"): {ProfileID: "viewer", ExpiresAt: now - 1}},
		persist: func(file string, value any) error {
			attempts[file]++
			if failing {
				return errors.New("forced state write failure")
			}
			data, err := json.Marshal(value)
			persisted[file] = string(data)
			return err
		},
		sessionTimeouts: func() (time.Duration, time.Duration) { return 15 * time.Minute, 8 * time.Hour },
	}
	for _, token := range []string{"expired-token", "stale-public-token", "expired-key"} {
		if _, ok := store.access(accessStateRequest(t, token)); ok {
			t.Fatalf("%s was accepted", token)
		}
	}
	if attempts["sessions.json"] != 2 || attempts["api_keys.json"] != 1 || len(store.sessions) != 1 || len(store.apiKeys) != 0 {
		t.Fatalf("failed deletions attempts=%v sessions=%d apiKeys=%d", attempts, len(store.sessions), len(store.apiKeys))
	}
	for _, operation := range []string{"session-expire", "session-revoke", "api-key-expire"} {
		requireAccessStateLog(t, output.String(), "WARN", operation, "retry-scheduled")
	}
	for range 5 {
		if _, ok := store.access(accessStateRequest(t, "active-token")); !ok {
			t.Fatal("active session was rejected during the write outage")
		}
	}
	if attempts["sessions.json"] != 2 || attempts["api_keys.json"] != 1 {
		t.Fatalf("requests inside the retry window wrote state: %v", attempts)
	}
	failing = false
	for file := range store.accessRetry {
		store.accessRetry[file] = now - 1
	}
	if _, ok := store.access(accessStateRequest(t, "active-token")); !ok || attempts["sessions.json"] != 2 || attempts["api_keys.json"] != 1 {
		t.Fatalf("request retried state writes: ok=%v attempts=%v", ok, attempts)
	}
	store.flushAccessState(t.Context())
	if attempts["sessions.json"] != 3 || attempts["api_keys.json"] != 2 || len(store.accessRetry) != 0 {
		t.Fatalf("retry attempts=%v pending=%v", attempts, store.accessRetry)
	}
	if strings.Contains(persisted["sessions.json"], sessionKey("expired-token")) || strings.Contains(persisted["sessions.json"], sessionKey("stale-public-token")) || persisted["api_keys.json"] != "{}" {
		t.Fatalf("retried deletions were not durable: %v", persisted)
	}
	requireAccessStateLog(t, output.String(), "INFO", "retry", "persisted")
	store.mu.Lock()
	active := store.sessions[sessionKey("active-token")]
	active.LastSeen = now - 61
	store.sessions[sessionKey("active-token")] = active
	store.apiKeys[sessionKey("active-key")] = apiKey{ProfileID: "viewer", ExpiresAt: now + 3600}
	store.mu.Unlock()
	for _, token := range []string{"active-token", "active-key", "active-token", "active-key"} {
		if _, ok := store.access(accessStateRequest(t, token)); !ok {
			t.Fatalf("%s was rejected", token)
		}
	}
	if attempts["sessions.json"] != 3 || attempts["api_keys.json"] != 2 {
		t.Fatalf("request path wrote use timestamps: %v", attempts)
	}
	store.flushAccessState(t.Context())
	store.flushAccessState(t.Context())
	if attempts["sessions.json"] != 4 || attempts["api_keys.json"] != 3 {
		t.Fatalf("use timestamps were not flushed once: %v", attempts)
	}
	var sessions map[string]viewerSession
	var keys map[string]apiKey
	if json.Unmarshal([]byte(persisted["sessions.json"]), &sessions) != nil || sessions[sessionKey("active-token")].LastSeen < now || json.Unmarshal([]byte(persisted["api_keys.json"]), &keys) != nil || keys[sessionKey("active-key")].LastUsed < now {
		t.Fatalf("flushed use timestamps = %v", persisted)
	}
	for _, secret := range []string{"expired-token", "stale-public-token", "active-token", "expired-key", sessionKey("expired-token"), sessionKey("active-token"), sessionKey("expired-key")} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("access state log exposed %q: %s", secret, output.String())
		}
	}
}

func TestAccessUseIsFlushedWhenTheLifecycleEnds(t *testing.T) {
	now := time.Now().Unix()
	persisted := map[string]string{}
	store := &profileStore{
		file: "profiles.json", sessionFile: "sessions.json", apiFile: "api_keys.json",
		profiles: []viewerProfile{{ID: "viewer"}},
		sessions: map[string]viewerSession{sessionKey("browser-token"): {ProfileID: "viewer", Browser: true, CreatedAt: now - 120, LastSeen: now - 120, ExpiresAt: now + 3600}},
		apiKeys:  map[string]apiKey{},
		persist: func(file string, value any) error {
			data, err := json.Marshal(value)
			persisted[file] = string(data)
			return err
		},
		sessionTimeouts: func() (time.Duration, time.Duration) { return 15 * time.Minute, 8 * time.Hour },
	}
	lifecycle, end := context.WithCancel(t.Context())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		store.runAccessFlush(lifecycle, time.Hour)
	}()
	if _, ok := store.access(accessStateRequest(t, "browser-token")); !ok {
		t.Fatal("browser session was rejected")
	}
	end()
	<-stopped
	var sessions map[string]viewerSession
	if json.Unmarshal([]byte(persisted["sessions.json"]), &sessions) != nil || sessions[sessionKey("browser-token")].LastSeen < now {
		t.Fatalf("lifecycle end did not flush session use: %v", persisted)
	}
}

func accessStateRequest(t *testing.T, token string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	return request
}

func requireAccessStateLog(t *testing.T, output, level, operation, outcome string) {
	t.Helper()
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		var record map[string]any
		if json.Unmarshal([]byte(line), &record) == nil && record["level"] == level && record["operation"] == operation && record["outcome"] == outcome && record["document"] != nil {
			return
		}
	}
	t.Fatalf("missing %s %s %s log: %s", level, operation, outcome, output)
}
