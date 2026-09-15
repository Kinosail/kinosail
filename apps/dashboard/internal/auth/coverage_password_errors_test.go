package auth

import (
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

type coverageUnavailableEntropy struct{}

func (coverageUnavailableEntropy) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestCoverageBcryptEntropyFailurePreservesState(t *testing.T) {
	manager, primary, _ := managerWithTwoBrowserSessions(t)
	beforeOwner, beforeSessions := manager.owner, append([]sessionState(nil), manager.sessions...)
	unconfigured, store := coverageSessionManager(t)
	unconfigured.owner = ownerState{}
	reader := rand.Reader
	rand.Reader = coverageUnavailableEntropy{}
	t.Cleanup(func() { rand.Reader = reader })
	if _, err := unconfigured.Setup(t.Context(), "Owner", currentTestPassword, "Browser"); !errors.Is(err, ErrState) {
		t.Fatalf("setup salt failure = %v", err)
	}
	if unconfigured.Configured() || len(store.documents) != 0 {
		t.Fatal("failed salt generation saved owner")
	}
	if err := manager.ChangePassword(t.Context(), currentTestPassword, newTestPassword, newTestPassword, primary.Token); !errors.Is(err, ErrState) {
		t.Fatalf("password salt failure = %v", err)
	}
	if !reflect.DeepEqual(manager.owner, beforeOwner) || !reflect.DeepEqual(manager.sessions, beforeSessions) {
		t.Fatal("failed salt generation changed authentication")
	}
}

func TestCoverageLogoutKeepsOtherSessions(t *testing.T) {
	manager, _ := coverageSessionManager(t)
	first, firstState := manager.newSession("First", "browser")
	_, secondState := manager.newSession("Second", "browser")
	manager.sessions = []sessionState{firstState, secondState}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	request.Header.Set("Authorization", "Bearer "+first.Token)
	if err := manager.Logout(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(manager.sessions, []sessionState{secondState}) {
		t.Fatal("logout changed another session")
	}
	if err := manager.Logout(t.Context(), nil); err != nil {
		t.Fatalf("anonymous logout = %v", err)
	}
}

func TestCoverageUnconfiguredLoginAndPassword(t *testing.T) {
	manager, _ := coverageSessionManager(t)
	manager.owner = ownerState{}
	if _, err := manager.Login(t.Context(), "Owner", currentTestPassword, ""); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("invalid device = %v", err)
	}
	if _, err := manager.Login(t.Context(), "Owner", currentTestPassword, "Browser"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("unconfigured login = %v", err)
	}
	if err := manager.ChangePassword(t.Context(), currentTestPassword, newTestPassword, newTestPassword, strings.Repeat("a", 64)); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("unconfigured password = %v", err)
	}
}
