package homeassistant

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type testProfile struct{ ID string }

type failingReader struct{ err error }

func (reader failingReader) Read([]byte) (int, error) { return 0, reader.err }

type testState struct {
	enabled              bool
	saved                []bool
	revoked, created     int
	saveErr, revokeErr   error
	createErr, browseErr error
	profile              Profile[testProfile]
	items                map[string]Item
	safe                 bool
	now                  time.Time
	server               Server
}

func newTestIntegration(t *testing.T, state *testState) *Integration[testProfile] {
	t.Helper()
	if state.now.IsZero() {
		state.now = time.Unix(1_700_000_000, 0)
	}
	if state.server == (Server{}) {
		state.server = Server{"Test Server", "server-id"}
	}
	if state.items == nil {
		state.items = map[string]Item{"item": {ID: "item"}}
	}
	config := Config[testProfile]{
		Random:  bytes.NewReader(make([]byte, 512)),
		Enabled: func() bool { return state.enabled },
		SaveEnabled: func(enabled bool) error {
			state.saved = append(state.saved, enabled)
			if state.saveErr == nil {
				state.enabled = enabled
			}
			return state.saveErr
		},
		RevokeKeys: func() error { state.revoked++; return state.revokeErr },
		CreateKey: func(profile testProfile, _ string) (string, error) {
			state.created++
			if state.createErr != nil {
				return "", state.createErr
			}
			return "token-" + profile.ID, nil
		},
		FindProfile: func(id string) (Profile[testProfile], bool) {
			return state.profile, id == state.profile.ID
		},
		CurrentProfile: func(*http.Request) Profile[testProfile] { return state.profile },
		Server:         func() Server { return state.server },
		TrustedHTTPS:   func() TrustedHTTPS { return TrustedHTTPS{} },
		Browse: func(*http.Request) (Library, error) {
			return Library{Items: []string{"item"}, View: "all", Total: 1, Limit: 24}, state.browseErr
		},
		VisibleItem: func(_ *http.Request, id string) (Item, bool) {
			item, found := state.items[id]
			return item, found
		},
		FindItem: func(id string) (Item, bool) {
			item, found := state.items[id]
			return item, found
		},
		SafePath: func(string) bool { return state.safe },
	}
	integration, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	integration.now = func() time.Time { return state.now }
	integration.text = func() string { return "random-text" }
	return integration
}

func request(method, target string, body io.Reader) *http.Request {
	result := httptest.NewRequestWithContext(context.Background(), method, target, body)
	if body != nil {
		result.Header.Set("Content-Type", "application/json")
	}
	return result
}

func formRequest(method, target string, values url.Values) *http.Request {
	result := httptest.NewRequestWithContext(context.Background(), method, target, strings.NewReader(values.Encode()))
	result.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return result
}

func response(handler http.Handler, request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestLifecycleAndSettings(t *testing.T) { //nolint:cyclop // One lifecycle matrix covers startup and every state-transition failure.
	failed := Config[testProfile]{Random: strings.NewReader("short")}
	failed.Enabled = func() bool { return true }
	if _, err := New(failed); err == nil || !strings.Contains(err.Error(), "media secret") {
		t.Fatalf("New short random error = %v", err)
	}

	state := &testState{revokeErr: errors.New("revoke")}
	if _, err := New(Config[testProfile]{Random: bytes.NewReader(make([]byte, 32)), Enabled: func() bool { return false }, RevokeKeys: func() error { return state.revokeErr }}); err == nil || !strings.Contains(err.Error(), "revoke Home Assistant keys") {
		t.Fatalf("New revoke error = %v", err)
	}

	state = &testState{enabled: true}
	integration := newTestIntegration(t, state)
	defaultRandom := integration.config
	defaultRandom.Random = nil
	if _, err := New(defaultRandom); err != nil {
		t.Fatalf("New default random = %v", err)
	}
	if !integration.Enabled() {
		t.Fatal("Enabled = false")
	}
	state.revokeErr = errors.New("revoke")
	if err := integration.SetEnabled(false); !errors.Is(err, state.revokeErr) || len(state.saved) != 0 {
		t.Fatalf("SetEnabled revoke = %v, saves %v", err, state.saved)
	}
	state.revokeErr = nil
	state.saveErr = errors.New("save")
	if err := integration.SetEnabled(false); !errors.Is(err, state.saveErr) || !state.enabled {
		t.Fatalf("SetEnabled save = %v, enabled %v", err, state.enabled)
	}
	state.saveErr = nil
	integration.pairs["pair"] = pairing[testProfile]{}
	integration.players["player"] = playerRecord{}
	integration.requests["request"] = authorization{}
	integration.codes["code"] = authorization{}
	if err := integration.SetEnabled(false); err != nil || state.enabled || len(integration.pairs)+len(integration.players)+len(integration.requests)+len(integration.codes) != 0 {
		t.Fatalf("SetEnabled(false) = %v, state %#v", err, integration)
	}
	if err := integration.SetEnabled(true); err != nil || !state.enabled {
		t.Fatalf("SetEnabled(true) = %v", err)
	}
	called := false
	integration.SetPublisher(func(_, _, _ string) { called = true })
	integration.publish("", "", "")
	if !called {
		t.Fatal("publisher not installed")
	}
}

func TestGateAndSettingsHTTP(t *testing.T) { //nolint:cyclop,funlen // One transport matrix proves both settings adapters.
	enabled := false
	gate := Gate(func() bool { return enabled }, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, test := range []struct {
		path string
		want int
	}{
		{"/api/v1/home-assistant", http.StatusNotFound},
		{"/home-assistant/authorize", http.StatusNotFound},
		{"/other", http.StatusNoContent},
	} {
		got := response(gate, request(http.MethodGet, test.path, nil))
		if got.Code != test.want {
			t.Errorf("Gate(%q) = %d, want %d", test.path, got.Code, test.want)
		}
	}
	enabled = true
	if got := response(gate, request(http.MethodGet, "/api/v1/home-assistant", nil)); got.Code != http.StatusNoContent {
		t.Fatalf("enabled gate = %d", got.Code)
	}

	state := &testState{enabled: true}
	integration := newTestIntegration(t, state)
	for _, test := range []struct {
		body string
		want int
	}{
		{`{}`, http.StatusBadRequest},
		{`{"enabled":false,"extra":true}`, http.StatusBadRequest},
		{`{"enabled":false}`, http.StatusOK},
	} {
		got := response(integration.SettingHandler(), request(http.MethodPost, "/", strings.NewReader(test.body)))
		if got.Code != test.want {
			t.Errorf("SettingHandler(%s) = %d: %s", test.body, got.Code, got.Body.String())
		}
	}
	state.saveErr = errors.New("save")
	got := response(integration.SettingHandler(), request(http.MethodPost, "/", strings.NewReader(`{"enabled":true}`)))
	if got.Code != http.StatusBadRequest {
		t.Fatalf("SettingHandler save = %d", got.Code)
	}

	writeError := func(w http.ResponseWriter, _ *http.Request, message string, status int) {
		http.Error(w, message, status)
	}
	for _, test := range []struct {
		contentType string
		values      url.Values
		want        int
	}{
		{"text/plain", url.Values{}, http.StatusBadRequest},
		{"application/x-www-form-urlencoded", url.Values{"enabled": {"false"}}, http.StatusBadRequest},
		{"application/x-www-form-urlencoded", url.Values{}, http.StatusBadRequest},
	} {
		req := formRequest(http.MethodPost, "/", test.values)
		req.Header.Set("Content-Type", test.contentType)
		got = response(integration.SaveSetting("/settings", writeError), req)
		if got.Code != test.want {
			t.Errorf("SaveSetting(%v) = %d", test.values, got.Code)
		}
	}
	state.saveErr = nil
	got = response(integration.SaveSetting("/settings", writeError), formRequest(http.MethodPost, "/", url.Values{}))
	if got.Code != http.StatusSeeOther || state.enabled {
		t.Fatalf("SaveSetting disabled = %d, enabled %v", got.Code, state.enabled)
	}
	got = response(integration.SaveSetting("/settings", writeError), formRequest(http.MethodPost, "/", url.Values{"enabled": {"true"}}))
	if got.Code != http.StatusSeeOther || got.Header().Get("Location") != "/settings" {
		t.Fatalf("SaveSetting success = %d %q", got.Code, got.Header().Get("Location"))
	}
}
