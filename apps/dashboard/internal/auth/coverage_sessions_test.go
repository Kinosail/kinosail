package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func coverageSessionManager(t *testing.T) (*Manager, *fakeDocumentStore) {
	t.Helper()
	store := newFakeDocumentStore(t, nil)
	manager := &Manager{store: store, now: time.Now, owner: ownerState{ID: strings.Repeat("1", 24), Name: "Owner"}}
	return manager, store
}

func TestCoverageMCPTokenLifecycle(t *testing.T) {
	manager, store := coverageSessionManager(t)
	manager.owner.ID = ""
	if _, err := manager.IssueMCPToken(t.Context()); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("unconfigured token = %v", err)
	}
	manager.owner.ID = strings.Repeat("1", 24)
	manager.sessions = make([]sessionState, maxSessions)
	token, err := manager.IssueMCPToken(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if token.Token == "" || len(manager.sessions) != maxSessions || manager.sessions[0].Purpose != "mcp" {
		t.Fatalf("issued token = %+v", token)
	}
	assertCoverageMCPStorageRollback(t, manager, store)
}

func TestCoverageAuthenticationPrunesExpiredSessions(t *testing.T) {
	for _, matched := range []bool{false, true} {
		t.Run(map[bool]string{false: "unknown", true: "valid"}[matched], func(t *testing.T) {
			manager, store := coverageSessionManager(t)
			token := strings.Repeat("a", 64)
			manager.sessions = []sessionState{
				{TokenHash: hashToken(token), CSRF: strings.Repeat("b", 48), ExpiresAt: time.Now().Add(time.Hour)},
				{TokenHash: strings.Repeat("c", 64), CSRF: strings.Repeat("d", 48), ExpiresAt: time.Now().Add(-time.Hour)},
			}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if !matched {
				token = strings.Repeat("e", 64)
			}
			request.AddCookie(&http.Cookie{Name: SessionCookie, Value: token}) // #nosec G124 -- Incoming test cookie; request cookies carry no server security attributes.
			if _, ok := manager.Authenticate(t.Context(), request); ok != matched {
				t.Fatalf("authentication = %v", ok)
			}
			if len(manager.sessions) != 1 || len(store.documents[sessionsDocument]) == 0 {
				t.Fatal("expired session was not pruned and saved")
			}
		})
	}
}

func TestCoverageAuthenticationRejectsInvalidTransport(t *testing.T) {
	manager, _ := coverageSessionManager(t)
	if _, ok := manager.Authenticate(t.Context(), nil); ok {
		t.Fatal("nil request authenticated")
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	if _, ok := manager.Authenticate(t.Context(), request); ok {
		t.Fatal("missing token authenticated")
	}
	request.AddCookie(&http.Cookie{Name: SessionCookie, Value: "invalid"}) // #nosec G124 -- Incoming malformed test cookie, not a server-issued cookie.
	if _, ok := manager.Authenticate(t.Context(), request); ok {
		t.Fatal("malformed token authenticated")
	}
}

func TestCoverageSessionValidation(t *testing.T) {
	now := time.Now()
	valid := sessionState{TokenHash: strings.Repeat("a", 64), CSRF: strings.Repeat("b", 48), CreatedAt: now, ExpiresAt: now.Add(time.Hour), Device: "Browser"}
	for _, field := range []string{"device", "purpose"} {
		session := valid
		if field == "device" {
			session.Device = ""
		} else {
			session.Purpose = "unknown"
		}
		if err := validateSession(session); err == nil {
			t.Fatalf("invalid %s accepted", field)
		}
	}
	for _, values := range [][3]string{{"", "valid-password", "Browser"}, {"Owner", "valid-password", ""}, {"Owner", "bad", "Browser"}} {
		if _, _, _, err := validateCredentials(values[0], values[1], values[2]); err == nil {
			t.Fatal("invalid credentials accepted")
		}
	}
	if _, err := boundedText("name", "Ow\x00ner", 60); err == nil {
		t.Fatal("control character accepted")
	}
}

func assertCoverageMCPStorageRollback(t *testing.T, manager *Manager, store *fakeDocumentStore) {
	t.Helper()
	before := append([]sessionState(nil), manager.sessions...)
	store.failSave = true
	if _, err := manager.IssueMCPToken(t.Context()); !errors.Is(err, ErrState) || !reflect.DeepEqual(before, manager.sessions) {
		t.Fatal("failed issue did not restore sessions")
	}
	if err := manager.RevokeMCPToken(t.Context()); err == nil || !reflect.DeepEqual(before, manager.sessions) {
		t.Fatal("failed revocation did not restore sessions")
	}
	store.failSave = false
	if err := manager.RevokeMCPToken(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(manager.sessions) != maxSessions-1 {
		t.Fatalf("remaining sessions = %d", len(manager.sessions))
	}
	store.failSave = true
	if err := manager.RevokeMCPToken(t.Context()); err != nil {
		t.Fatalf("empty revocation wrote storage: %v", err)
	}
}
