package remoteaccess

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/wireguard"
)

func TestKillHTTPResultPaths(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/remote/kill", nil)
	tests := []struct {
		name       string
		manager    func(*testing.T) *Manager
		revoke     func() error
		wantStatus int
	}{
		{"missing", func(*testing.T) *Manager { return nil }, func() error { return nil }, http.StatusConflict},
		{"kill failure", func(*testing.T) *Manager { manager, _ := New(Config{}); return manager }, func() error { return nil }, http.StatusConflict},
		{"revoke failure", func(t *testing.T) *Manager { return activeManager(t, true) }, func() error { return errors.New("revoke failed") }, http.StatusInternalServerError},
		{"success", func(t *testing.T) *Manager { return activeManager(t, true) }, func() error { return nil }, http.StatusSeeOther},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			KillHTTP(test.manager(t), test.revoke, plainHTTPError)(recorder, request)
			if recorder.Code != test.wantStatus || test.wantStatus == http.StatusSeeOther && recorder.Header().Get("Location") != "/settings#access" {
				t.Fatalf("response = %d %q", recorder.Code, recorder.Header().Get("Location"))
			}
		})
	}
}

func TestResetKillHTTPResultPaths(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/remote/enable", nil)
	disabled, _ := New(Config{})
	active := activeManager(t, true)
	if err := active.Kill(); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name       string
		manager    *Manager
		wantStatus int
	}{{"missing", nil, http.StatusConflict}, {"disabled", disabled, http.StatusConflict}, {"success", active, http.StatusSeeOther}} {
		recorder := httptest.NewRecorder()
		ResetKillHTTP(test.manager, plainHTTPError)(recorder, request)
		if recorder.Code != test.wantStatus || test.wantStatus == http.StatusSeeOther && recorder.Header().Get("Location") != "/settings#access" {
			t.Fatalf("%s response = %d %q", test.name, recorder.Code, recorder.Header().Get("Location"))
		}
	}
}

func plainHTTPError(writer http.ResponseWriter, _ *http.Request, message string, status int) {
	http.Error(writer, message, status)
}

func TestStatusAndMutationAPIs(t *testing.T) { //nolint:cyclop,funlen // The paired endpoint contract verifies every status and failure translation.
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/remote-access", nil)
	var payload map[string]any
	write := func(writer http.ResponseWriter, value any, status int) {
		payload = value.(map[string]any)
		writer.WriteHeader(status)
	}
	response := httptest.NewRecorder()
	StatusAPI(nil, nil, func(status Status) string { return status.State }, write)(response, request)
	if response.Code != http.StatusOK || payload["securePublic"] != "disabled" || payload["verifiedDirect"].(map[string]any)["enabled"] != false {
		t.Fatalf("disabled status = %d %#v", response.Code, payload)
	}
	verified, err := wireguard.Open(t.TempDir(), "media.example.com:51820")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = verified.Pair("Phone", "viewer"); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	StatusAPI(verified, activeManager(t, true), func(status Status) string { return status.State }, write)(response, request)
	if payload["verifiedDirect"].(map[string]any)["enabled"] != true || len(payload["verifiedDirect"].(map[string]any)["peers"].([]wireguard.Viewer)) != 1 {
		t.Fatalf("active status = %#v", payload)
	}

	contract := func(message string) error { return errors.New("contract: " + message) }
	writeAPIError := func(writer http.ResponseWriter, err error, status int) { http.Error(writer, err.Error(), status) }
	response = httptest.NewRecorder()
	KillAPI(nil, func() error { return nil }, contract, writeAPIError)(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "contract:") {
		t.Fatalf("kill conflict = %d %q", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	KillAPI(activeManager(t, true), func() error { return errors.New("revoke") }, contract, writeAPIError)(response, request)
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "contract:") {
		t.Fatalf("kill revoke = %d %q", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	KillAPI(activeManager(t, true), func() error { return nil }, contract, writeAPIError)(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("kill success = %d", response.Code)
	}
	response = httptest.NewRecorder()
	ResetKillAPI(nil, contract, writeAPIError)(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "contract:") {
		t.Fatalf("reset conflict = %d %q", response.Code, response.Body.String())
	}
	manager := activeManager(t, true)
	if manager.Kill() != nil {
		t.Fatal("could not prepare reset")
	}
	response = httptest.NewRecorder()
	ResetKillAPI(manager, contract, writeAPIError)(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("reset success = %d", response.Code)
	}
}

func TestRevokeAuthorizationJoinsDurableFailures(t *testing.T) {
	t.Parallel()
	var passkeys, quick bool
	profileErr, shareErr := errors.New("profiles"), errors.New("shares")
	err := RevokeAuthorization(func() { passkeys = true }, func() { quick = true }, func() error { return profileErr }, func() error { return shareErr })
	if !passkeys || !quick || !errors.Is(err, profileErr) || !errors.Is(err, shareErr) {
		t.Fatalf("revocation = %t %t %v", passkeys, quick, err)
	}
	if RevokeAuthorization(func() {}, func() {}, func() error { return nil }, nil) != nil {
		t.Fatal("empty revocation failed")
	}
}
