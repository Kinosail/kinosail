package owneraccess

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestManagementFormAndJSONRejectionHasNoStateEffects(t *testing.T) {
	config, _, _ := ownerFixture(t)
	m, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	h := HTTP{Manager: m, OwnerID: func(*http.Request) string { return "owner" }, Error: func(w http.ResponseWriter, _ *http.Request, message string, status int) {
		http.Error(w, message, status)
	}, JSON: func(http.ResponseWriter, any, int) { t.Fatal("invalid request succeeded") }}
	for _, raw := range []string{`null`, `{"endpoint":"one.example:51821","endpoint":"two.example:51821"}`, `{"endpoint":"one.example:51821","owner":true}`, `{"endpoint":123}`, strings.Repeat(" ", 4097)} {
		r := httptest.NewRequestWithContext(t.Context(), "POST", "/api/v1/management-access", strings.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.Change("enable", true).ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("JSON status %d", w.Code)
		}
	}
	for _, raw := range []string{"endpoint=a.example%3A51821&endpoint=b.example%3A51821", "endpoint=a.example%3A51821&unknown=x", "endpoint=%", "label=phone"} {
		r := httptest.NewRequestWithContext(t.Context(), "POST", "/settings/management/enable", strings.NewReader(raw))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		h.Change("enable", false).ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("form status %d", w.Code)
		}
	}
	entries, _ := os.ReadDir(config.Directory)
	if len(entries) != 0 {
		t.Fatal("rejection wrote state")
	}
}

func TestManagementFailedSaveKeepsRecoveryClosed(t *testing.T) {
	config, _, _ := ownerFixture(t)
	m, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	m.write = func(string, []byte) error { return errors.New("disk failure") }
	if err = m.Disable(); err == nil {
		t.Fatal("failed save reported success")
	}
	reopened, err := Open(config)
	if err != nil || reopened.Status().Enabled {
		t.Fatal("failed save reopened management")
	}
}

func TestManagementDisableRejectsWrongEncodingAndAmbiguousFormWithoutSaving(t *testing.T) {
	config, _, _ := ownerFixture(t)
	m, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	writes := 0
	m.write = func(string, []byte) error { writes++; return nil }
	h := HTTP{Manager: m, Error: func(w http.ResponseWriter, _ *http.Request, message string, status int) {
		http.Error(w, message, status)
	}}
	for _, input := range []struct{ contentType, body, query string }{
		{"application/json", `{"unknown":true}`, ""},
		{"", "", ""},
		{"application/x-www-form-urlencoded", "unknown=x", ""},
		{"application/x-www-form-urlencoded", "_csrf=a&_csrf=b", ""},
		{"application/x-www-form-urlencoded", "", "?unknown=x"},
		{"application/x-www-form-urlencoded", strings.Repeat("x", 4097), ""},
	} {
		r := httptest.NewRequestWithContext(t.Context(), "POST", "/settings/management/disable"+input.query, strings.NewReader(input.body))
		r.Header.Set("Content-Type", input.contentType)
		w := httptest.NewRecorder()
		h.Change("disable", false).ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest || writes != 0 {
			t.Fatal("invalid disable caused effects")
		}
	}
	entries, err := os.ReadDir(config.Directory)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid disable wrote recovery marker")
	}
}

func TestManagementFailedPairDoesNotReturnKeysAndFailedRevokeStaysClosed(t *testing.T) {
	config, _, _ := ownerFixture(t)
	m, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	m.state.Enabled = true
	m.runtime = &runtime{used: map[string]bool{}}
	m.write = func(string, []byte) error { return errors.New("disk failure") }
	if profile, err := m.Pair("Phone", "owner"); err == nil || profile != "" || len(m.state.Devices) != 0 {
		t.Fatal("failed pairing exposed keys or changed peers")
	}
	m.runtime = nil
	key := strings.Repeat("A", 43) + "="
	m.state.Devices = []Device{{PublicKey: key}}
	if m.Revoke(key) == nil {
		t.Fatal("failed revocation reported success")
	}
	reopened, err := Open(config)
	if err != nil || reopened.Status().Enabled {
		t.Fatal("revoked device returned after failed save")
	}
}

func TestManagementOriginUsesCanonicalHTTPSPort(t *testing.T) {
	config, _, _ := ownerFixture(t)
	config.Origin += ":443"
	m, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"server.example", "server.example:443"} {
		if !managementHost(host, m.config.Origin) {
			t.Fatal("equivalent HTTPS Host rejected")
		}
	}
	for _, host := range []string{"server.example:444", "attacker.example", "server.example:0443", "server.example@attacker.example"} {
		if managementHost(host, m.config.Origin) {
			t.Fatal("foreign Host accepted")
		}
	}
}
