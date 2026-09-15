package wireguard_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/wireguard"
)

type profileDirectory map[string]wireguard.Profile

func (profiles profileDirectory) WireGuardProfile(id string) (wireguard.Profile, bool) {
	profile, found := profiles[id]
	return profile, found
}

type httpEffects struct {
	payload map[string]string
	read    bool
}

func wireHTTP(effects *httpEffects, manager *wireguard.Manager, profiles profileDirectory) wireguard.HTTPConfig {
	return wireguard.NewHTTP(manager, profiles, func(_ http.ResponseWriter, _ *http.Request, target any) bool {
		if !effects.read {
			return false
		}
		data, _ := json.Marshal(effects.payload)
		return json.Unmarshal(data, target) == nil
	}, func(writer http.ResponseWriter, value any, status int) {
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(value)
	}, func(writer http.ResponseWriter, err error, status int) {
		http.Error(writer, err.Error(), status)
	}, func(writer http.ResponseWriter) {
		http.Error(writer, "not found", http.StatusNotFound)
	}, func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
		http.Error(writer, message, status)
	})
}

func TestPairingAPIHandlers(t *testing.T) {
	t.Parallel()
	profiles := profileDirectory{"viewer": {ID: "viewer", Owner: false}, "owner": {ID: "owner", Owner: true}}
	for _, test := range []struct {
		name       string
		read       bool
		manager    bool
		profile    string
		label      string
		wantStatus int
	}{
		{"decode", false, true, "viewer", "Phone", http.StatusOK},
		{"manager", true, false, "viewer", "Phone", http.StatusNotFound},
		{"profile", true, true, "missing", "Phone", http.StatusBadRequest},
		{"owner", true, true, "owner", "Phone", http.StatusBadRequest},
		{"pair", true, true, "viewer", "", http.StatusBadRequest},
		{"success", true, true, "viewer", "Phone", http.StatusCreated},
	} {
		t.Run(test.name, func(t *testing.T) {
			var manager *wireguard.Manager
			if test.manager {
				manager = openWireGuard(t)
			}
			effects := &httpEffects{read: test.read, payload: map[string]string{"Label": test.label, "ProfileID": test.profile}}
			recorder := httptest.NewRecorder()
			wireHTTP(effects, manager, profiles).CreateAPI()(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))
			if recorder.Code != test.wantStatus || test.wantStatus == http.StatusCreated && !strings.Contains(recorder.Body.String(), "[Interface]") {
				t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
			}
		})
	}

	manager := openWireGuard(t)
	pairing, err := manager.Pair("Phone", "viewer")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, key  string
		read       bool
		manager    *wireguard.Manager
		wantStatus int
	}{{"decode", pairing.ViewerPublicKey, false, manager, http.StatusOK}, {"manager", pairing.ViewerPublicKey, true, nil, http.StatusNotFound}, {"missing", strings.Repeat("A", 43) + "=", true, manager, http.StatusNotFound}, {"success", pairing.ViewerPublicKey, true, manager, http.StatusNoContent}} {
		effects := &httpEffects{read: test.read, payload: map[string]string{"PublicKey": test.key}}
		recorder := httptest.NewRecorder()
		wireHTTP(effects, test.manager, profiles).RevokeAPI()(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", nil))
		if recorder.Code != test.wantStatus {
			t.Fatalf("%s response = %d %q", test.name, recorder.Code, recorder.Body.String())
		}
	}
}

func TestPairingWebHandlers(t *testing.T) { //nolint:cyclop // One handler lifecycle verifies every pairing response.
	t.Parallel()
	profiles := profileDirectory{"viewer": {ID: "viewer"}, "owner": {ID: "owner", Owner: true}}
	for _, test := range []struct {
		name, profile, label string
		manager              bool
		wantStatus           int
	}{{"manager", "viewer", "Phone", false, http.StatusNotFound}, {"profile", "missing", "Phone", true, http.StatusBadRequest}, {"owner", "owner", "Phone", true, http.StatusBadRequest}, {"pair", "viewer", "", true, http.StatusBadRequest}, {"success", "viewer", "Phone", true, http.StatusCreated}} {
		var manager *wireguard.Manager
		if test.manager {
			manager = openWireGuard(t)
		}
		recorder := httptest.NewRecorder()
		request := formRequest(t, map[string]string{"profileId": test.profile, "label": test.label})
		wireHTTP(&httpEffects{}, manager, profiles).CreateWeb()(recorder, request)
		if recorder.Code != test.wantStatus || test.wantStatus == http.StatusCreated && (recorder.Header().Get("Content-Type") != "application/x-wireguard-profile" || !strings.Contains(recorder.Body.String(), "[Interface]")) {
			t.Fatalf("%s response = %d %q %v", test.name, recorder.Code, recorder.Body.String(), recorder.Header())
		}
	}

	manager := openWireGuard(t)
	pairing, err := manager.Pair("Phone", "viewer")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, key  string
		manager    *wireguard.Manager
		wantStatus int
	}{{"manager", pairing.ViewerPublicKey, nil, http.StatusNotFound}, {"missing", strings.Repeat("A", 43) + "=", manager, http.StatusNotFound}, {"success", pairing.ViewerPublicKey, manager, http.StatusSeeOther}} {
		recorder := httptest.NewRecorder()
		wireHTTP(&httpEffects{}, test.manager, profiles).RevokeWeb()(recorder, formRequest(t, map[string]string{"publicKey": test.key}))
		if recorder.Code != test.wantStatus || test.wantStatus == http.StatusSeeOther && recorder.Header().Get("Location") != "/settings" {
			t.Fatalf("%s response = %d %q", test.name, recorder.Code, recorder.Header().Get("Location"))
		}
	}
}

func openWireGuard(t *testing.T) *wireguard.Manager {
	t.Helper()
	manager, err := wireguard.Open(t.TempDir(), "media.example.com:51820")
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func formRequest(t *testing.T, values map[string]string) *http.Request {
	t.Helper()
	form := make(url.Values, len(values))
	for key, value := range values {
		form.Set(key, value)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}
