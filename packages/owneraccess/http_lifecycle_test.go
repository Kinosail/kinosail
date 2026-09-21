package owneraccess

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func managementHTTP(t *testing.T, manager *Manager) HTTP {
	t.Helper()
	return HTTP{Manager: manager, OwnerID: func(*http.Request) string { return "owner" }, JSON: func(w http.ResponseWriter, value any, status int) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(value); err != nil {
			t.Error(err)
		}
	}, Error: func(w http.ResponseWriter, _ *http.Request, message string, status int) {
		http.Error(w, message, status)
	}}
}

func managementChange(t *testing.T, adapter HTTP, operation string, api bool, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/management", strings.NewReader(body))
	contentType := "application/x-www-form-urlencoded"
	if api {
		contentType = "application/json"
	}
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()
	adapter.Change(operation, api).ServeHTTP(response, request)
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("management response permits caching or referral")
	}
	return response
}

func TestManagementHTTPUnavailableAndInvalidOperations(t *testing.T) {
	adapter := managementHTTP(t, nil)
	response := httptest.NewRecorder()
	adapter.Status(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/management", nil))
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("unavailable status exposed as success")
	}
	if result := managementChange(t, adapter, "enable", true, `{}`); result.Code != http.StatusServiceUnavailable {
		t.Fatal("unavailable mutation accepted")
	}
	config, _, _ := ownerFixture(t)
	manager, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	adapter = managementHTTP(t, manager)
	for _, api := range []bool{true, false} {
		body := ""
		if api {
			body = `{}`
		}
		if result := managementChange(t, adapter, "unknown", api, body); result.Code != http.StatusBadRequest {
			t.Fatal("unknown operation accepted")
		}
	}
}

func TestManagementHTTPEnablePairRevokeAndDisable(t *testing.T) {
	for _, api := range []bool{false, true} {
		name := "web"
		if api {
			name = "API"
		}
		t.Run(name, func(t *testing.T) { assertManagementHTTPLifecycle(t, api) })
	}
}

func assertManagementHTTPLifecycle(t *testing.T, api bool) {
	t.Helper()
	config, _, _ := ownerFixture(t)
	manager, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	manager.Attach(t.Context(), http.NotFoundHandler())
	t.Cleanup(func() { _ = manager.Disable() })
	adapter := managementHTTP(t, manager)
	enable, pair, disable := "endpoint=home.example%3A51821", "label=Phone", ""
	success := http.StatusSeeOther
	if api {
		enable, pair, disable, success = `{"endpoint":"home.example:51821"}`, `{"label":"Phone"}`, `{}`, http.StatusOK
	}
	if result := managementChange(t, adapter, "enable", api, enable); result.Code != success || !manager.Status().Enabled {
		t.Fatalf("enable=%d", result.Code)
	}
	paired := managementChange(t, adapter, "pair", api, pair)
	if paired.Code != http.StatusCreated || len(manager.Status().Devices) != 1 {
		t.Fatalf("pair=%d", paired.Code)
	}
	assertManagementPairResponse(t, paired, api)
	assertManagementStatusPrivate(t, adapter)
	key := manager.Status().Devices[0].PublicKey
	revoke := "publicKey=" + strings.ReplaceAll(strings.ReplaceAll(key, "+", "%2B"), "=", "%3D")
	if api {
		encoded, _ := json.Marshal(map[string]string{"publicKey": key})
		revoke = string(encoded)
	}
	assertManagementRevocation(t, adapter, manager, api, revoke, disable, pair, success)
}

func assertManagementStatusPrivate(t *testing.T, adapter HTTP) {
	t.Helper()
	response := httptest.NewRecorder()
	adapter.Status(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/management", nil))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "PrivateKey") || strings.Contains(response.Body.String(), "PresharedKey") {
		t.Fatal("status exposed private keys")
	}
}

func assertManagementRevocation(t *testing.T, adapter HTTP, manager *Manager, api bool, revoke, disable, pair string, success int) {
	t.Helper()
	if result := managementChange(t, adapter, "revoke", api, revoke); result.Code != success || len(manager.Status().Devices) != 0 {
		t.Fatalf("revoke=%d", result.Code)
	}
	if result := managementChange(t, adapter, "disable", api, disable); result.Code != success || manager.Status().Enabled {
		t.Fatalf("disable=%d", result.Code)
	}
	if result := managementChange(t, adapter, "pair", api, pair); result.Code != http.StatusBadRequest {
		t.Fatal("disabled management paired a device")
	}
}

func assertManagementPairResponse(t *testing.T, response *httptest.ResponseRecorder, api bool) {
	t.Helper()
	profile := response.Body.String()
	if api {
		var body map[string]string
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		profile = body["profile"]
	} else if response.Header().Get("Content-Type") != "application/x-wireguard-profile" || response.Header().Get("Content-Disposition") != `attachment; filename="kinosail-owner.conf"` {
		t.Fatal("profile not served as attachment")
	}
	if !strings.Contains(profile, "[Interface]") || !strings.Contains(profile, "[Peer]") {
		t.Fatal("pairing omitted WireGuard profile")
	}
}
