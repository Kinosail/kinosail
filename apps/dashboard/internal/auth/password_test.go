package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	currentTestPassword = "correct horse battery"
	newTestPassword     = "another correct horse"
)

func TestChangePasswordPreservesPresentedBrowserSessionAndRevokesOthers(t *testing.T) {
	manager, primary, other := managerWithTwoBrowserSessions(t)
	mcpSession, err := manager.IssueMCPToken(context.Background())
	if err != nil {
		t.Fatalf("IssueMCPToken: %v", err)
	}
	if err := manager.ChangePassword(context.Background(), currentTestPassword, newTestPassword, newTestPassword, primary.Token); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if !authenticatedWithCookie(t, manager, primary.Token) {
		t.Fatal("presented browser session was revoked")
	}
	if authenticatedWithCookie(t, manager, other.Token) {
		t.Fatal("other browser session remained active")
	}
	if authenticatedWithBearer(t, manager, mcpSession.Token) {
		t.Fatal("MCP session remained active")
	}
	if _, err := manager.Login(context.Background(), "Owner", currentTestPassword, "old password browser"); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("old password login error = %v, want ErrInvalidCredential", err)
	}
	if _, err := manager.Login(context.Background(), "Owner", newTestPassword, "new password browser"); err != nil {
		t.Fatalf("new password login: %v", err)
	}
}

func TestChangePasswordRejectsInvalidInputWithoutSideEffects(t *testing.T) {
	tests := []struct {
		name         string
		current      string
		newPassword  string
		confirmation string
	}{
		{name: "wrong current password", current: "wrong password value", newPassword: newTestPassword, confirmation: newTestPassword},
		{name: "oversized current password", current: strings.Repeat("a", 73), newPassword: newTestPassword, confirmation: newTestPassword},
		{name: "invalid UTF-8 current password", current: string([]byte{0xff}), newPassword: newTestPassword, confirmation: newTestPassword},
		{name: "short new password", current: currentTestPassword, newPassword: "too short", confirmation: "too short"},
		{name: "oversized ASCII new password", current: currentTestPassword, newPassword: strings.Repeat("a", 73), confirmation: strings.Repeat("a", 73)},
		{name: "oversized UTF-8 new password", current: currentTestPassword, newPassword: strings.Repeat("界", 25), confirmation: strings.Repeat("界", 25)},
		{name: "invalid UTF-8 new password", current: currentTestPassword, newPassword: string([]byte{0xff, 0xfe, 0xfd, 0xfc, 0xfb, 0xfa, 0xf9, 0xf8, 0xf7, 0xf6, 0xf5, 0xf4}), confirmation: string([]byte{0xff, 0xfe, 0xfd, 0xfc, 0xfb, 0xfa, 0xf9, 0xf8, 0xf7, 0xf6, 0xf5, 0xf4})},
		{name: "confirmation mismatch", current: currentTestPassword, newPassword: newTestPassword, confirmation: "different secure password"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager, primary, other := managerWithTwoBrowserSessions(t)
			if err := manager.ChangePassword(context.Background(), test.current, test.newPassword, test.confirmation, primary.Token); err == nil {
				t.Fatal("ChangePassword error = nil, want rejection")
			}
			if !authenticatedWithCookie(t, manager, primary.Token) || !authenticatedWithCookie(t, manager, other.Token) {
				t.Fatal("rejected password change altered sessions")
			}
			if _, err := manager.Login(context.Background(), "Owner", currentTestPassword, "verification browser"); err != nil {
				t.Fatalf("current password changed after rejection: %v", err)
			}
		})
	}
}

func TestChangePasswordRollsBackOwnerAndSessionsWhenPersistenceFails(t *testing.T) {
	store := newFakeDocumentStore(t, nil)
	manager, err := NewManager(context.Background(), store)
	if err != nil {
		t.Fatal(err)
	}
	primary, err := manager.Setup(context.Background(), "Owner", currentTestPassword, "primary browser")
	if err != nil {
		t.Fatal(err)
	}
	other, err := manager.Login(context.Background(), "Owner", currentTestPassword, "other browser")
	if err != nil {
		t.Fatal(err)
	}
	store.failBatch = true
	if err := manager.ChangePassword(context.Background(), currentTestPassword, newTestPassword, newTestPassword, primary.Token); !errors.Is(err, ErrState) {
		t.Fatalf("ChangePassword error = %v, want ErrState", err)
	}
	store.failBatch = false
	if !authenticatedWithCookie(t, manager, primary.Token) || !authenticatedWithCookie(t, manager, other.Token) {
		t.Fatal("failed password change did not restore sessions")
	}
	if _, err := manager.Login(context.Background(), "Owner", currentTestPassword, "verification browser"); err != nil {
		t.Fatalf("failed password change did not restore password: %v", err)
	}
}

func managerWithTwoBrowserSessions(t *testing.T) (*Manager, Session, Session) {
	t.Helper()
	manager, err := NewManager(context.Background(), newFakeDocumentStore(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	primary, err := manager.Setup(context.Background(), "Owner", currentTestPassword, "primary browser")
	if err != nil {
		t.Fatal(err)
	}
	other, err := manager.Login(context.Background(), "Owner", currentTestPassword, "other browser")
	if err != nil {
		t.Fatal(err)
	}
	return manager, primary, other
}

func authenticatedWithCookie(t *testing.T, manager *Manager, token string) bool {
	request := httptest.NewRequestWithContext(t.Context(), "GET", "/api/v1/me", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookie, Value: token}) // #nosec G124 -- Incoming request cookie; AddCookie sends only its name and value.
	_, found := manager.Authenticate(context.Background(), request)
	return found
}

func authenticatedWithBearer(t *testing.T, manager *Manager, token string) bool {
	request := httptest.NewRequestWithContext(t.Context(), "GET", "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	_, found := manager.Authenticate(context.Background(), request)
	return found
}
