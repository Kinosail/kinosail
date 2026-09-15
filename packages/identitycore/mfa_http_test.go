package identitycore

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mfaHTTPState struct {
	read, setup, confirm, strong, verify, verifyOK, disable, webError, webConfirm, webDisable bool
	setupError, confirmError, strongError, disableError                                       error
	code                                                                                      string
	record                                                                                    recordedResponse
}

type webMFATest struct {
	name                string
	mutate              func(*mfaHTTPState)
	status              int
	message             string
	first, second, done bool
}

func (state *mfaHTTPState) config() MFAHTTPConfig {
	return MFAHTTPConfig{
		ReadJSON: func(writer http.ResponseWriter, request *http.Request, target any) bool {
			state.read = true
			if strings.Contains(request.URL.RawQuery, "read=false") {
				writer.WriteHeader(http.StatusBadRequest)
				return false
			}
			return json.NewDecoder(request.Body).Decode(target) == nil
		},
		Setup: func(*http.Request) (Enrollment, error) {
			state.setup = true
			return Enrollment{Secret: "secret", URI: "uri", RecoveryCodes: []string{"code"}}, state.setupError
		},
		Confirm: func(_ *http.Request, code string) error {
			state.confirm, state.code = true, code
			return state.confirmError
		},
		MarkStrong: func(*http.Request) error { state.strong = true; return state.strongError },
		Verify:     func(_ *http.Request, code string) bool { state.verify, state.code = true, code; return state.verifyOK },
		Disable:    func(*http.Request) error { state.disable = true; return state.disableError },
		Error: func(writer http.ResponseWriter, err error, status int) {
			state.record.message, state.record.status = err.Error(), status
			writer.WriteHeader(status)
		},
		JSON: func(writer http.ResponseWriter, value any, status int) {
			state.record.json, state.record.status = value, status
			writer.WriteHeader(status)
		},
		WebError: func(writer http.ResponseWriter, _ *http.Request, err error, status int) {
			state.webError = true
			state.record.message, state.record.status = err.Error(), status
			writer.WriteHeader(status)
		},
		WebConfirmSuccess: func(writer http.ResponseWriter, _ *http.Request) {
			state.webConfirm = true
			state.record.status = http.StatusSeeOther
			writer.WriteHeader(http.StatusSeeOther)
		},
		WebDisableSuccess: func(writer http.ResponseWriter, _ *http.Request) {
			state.webDisable = true
			state.record.status = http.StatusSeeOther
			writer.WriteHeader(http.StatusSeeOther)
		},
	}
}

func TestRegisterMFALifecycle(t *testing.T) { //nolint:gocognit // The table covers every endpoint ordering branch.
	t.Parallel()
	tests := []struct {
		name                                    string
		method, path, body                      string
		mutate                                  func(*mfaHTTPState)
		status                                  int
		setup, confirm, strong, verify, disable bool
	}{
		{"read rejection", http.MethodPost, "/api/v1/me/mfa/setup?read=false", `{}`, func(*mfaHTTPState) {}, http.StatusBadRequest, false, false, false, false, false},
		{"setup failure", http.MethodPost, "/api/v1/me/mfa/setup", `{}`, func(state *mfaHTTPState) { state.setupError = errors.New("failed") }, http.StatusInternalServerError, true, false, false, false, false},
		{"setup", http.MethodPost, "/api/v1/me/mfa/setup", `{}`, func(*mfaHTTPState) {}, http.StatusCreated, true, false, false, false, false},
		{"confirm failure", http.MethodPut, "/api/v1/me/mfa", `{"code":"000000"}`, func(state *mfaHTTPState) { state.confirmError = errors.New("failed") }, http.StatusBadRequest, false, true, false, false, false},
		{"strong failure", http.MethodPut, "/api/v1/me/mfa", `{"code":"000000"}`, func(state *mfaHTTPState) { state.strongError = errors.New("failed") }, http.StatusInternalServerError, false, true, true, false, false},
		{"confirm", http.MethodPut, "/api/v1/me/mfa", `{"code":"000000"}`, func(*mfaHTTPState) {}, http.StatusOK, false, true, true, false, false},
		{"verify failure", http.MethodDelete, "/api/v1/me/mfa", `{"code":"000000"}`, func(*mfaHTTPState) {}, http.StatusUnauthorized, false, false, false, true, false},
		{"disable failure", http.MethodDelete, "/api/v1/me/mfa", `{"code":"000000"}`, func(state *mfaHTTPState) { state.verifyOK = true; state.disableError = errors.New("failed") }, http.StatusConflict, false, false, false, true, true},
		{"disable", http.MethodDelete, "/api/v1/me/mfa", `{"code":"000000"}`, func(state *mfaHTTPState) { state.verifyOK = true }, http.StatusNoContent, false, false, false, true, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			state := &mfaHTTPState{}
			test.mutate(state)
			handlers, err := NewMFAHandlers(state.config())
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/v1/me/mfa/setup", handlers.Setup)
			mux.HandleFunc("PUT /api/v1/me/mfa", handlers.Confirm)
			mux.HandleFunc("DELETE /api/v1/me/mfa", handlers.Disable)
			request := httptest.NewRequestWithContext(t.Context(), test.method, test.path, strings.NewReader(test.body))
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != test.status || state.setup != test.setup || state.confirm != test.confirm || state.strong != test.strong || state.verify != test.verify || state.disable != test.disable {
				t.Fatalf("MFA response=%d operations=%v/%v/%v/%v/%v", response.Code, state.setup, state.confirm, state.strong, state.verify, state.disable)
			}
		})
	}
}

func TestNewMFAHandlersRejectsInvalidConfig(t *testing.T) {
	t.Parallel()
	handlers, err := NewMFAHandlers(MFAHTTPConfig{})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("invalid MFA config error = %v", err)
	}
	if handlers.Setup != nil || handlers.Confirm != nil || handlers.Disable != nil || handlers.WebConfirm != nil || handlers.WebDisable != nil {
		t.Fatal("invalid MFA config returned handlers")
	}
}

func TestWebMFADisableLifecycle(t *testing.T) {
	t.Parallel()
	tests := []webMFATest{
		{"invalid code", func(*mfaHTTPState) {}, http.StatusUnauthorized, "invalid authentication code", true, false, false},
		{"disable failure", func(state *mfaHTTPState) {
			state.verifyOK = true
			state.disableError = errors.New("owner must retain a passkey or authenticator")
		}, http.StatusConflict, "owner must retain a passkey or authenticator", true, true, false},
		{"success", func(state *mfaHTTPState) { state.verifyOK = true }, http.StatusSeeOther, "", true, true, true},
	}
	runWebMFATests(t, "/account/mfa/disable", "000000", tests, func(handlers MFAHandlers) http.HandlerFunc { return handlers.WebDisable }, func(state *mfaHTTPState, test webMFATest) bool {
		return state.code == "000000" && state.verify == test.first && state.disable == test.second && state.webDisable == test.done && state.webError != test.done
	})
}

func TestWebMFAConfirmLifecycle(t *testing.T) {
	t.Parallel()
	tests := []webMFATest{
		{"invalid code", func(state *mfaHTTPState) {
			state.confirmError = errors.New("authentication code is invalid or enrollment expired")
		}, http.StatusBadRequest, "authentication code is invalid or enrollment expired", true, false, false},
		{"strong failure", func(state *mfaHTTPState) { state.strongError = errors.New("failed") }, http.StatusInternalServerError, "could not secure current session", true, true, false},
		{"success", func(*mfaHTTPState) {}, http.StatusSeeOther, "", true, true, true},
	}
	runWebMFATests(t, "/account/mfa/enable", "287082", tests, func(handlers MFAHandlers) http.HandlerFunc { return handlers.WebConfirm }, func(state *mfaHTTPState, test webMFATest) bool {
		return state.code == "287082" && state.confirm == test.first && state.strong == test.second && state.webConfirm == test.done && state.webError != test.done
	})
}

func runWebMFATests(t *testing.T, path, code string, tests []webMFATest, handler func(MFAHandlers) http.HandlerFunc, valid func(*mfaHTTPState, webMFATest) bool) {
	t.Helper()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			state, response := exerciseWebMFA(t, path, code, test.mutate, handler)
			if response.Code != test.status || state.record.message != test.message || !valid(state, test) {
				t.Fatalf("web MFA response=%d message=%q state=%#v", response.Code, state.record.message, state)
			}
		})
	}
}

func exerciseWebMFA(t *testing.T, path, code string, mutate func(*mfaHTTPState), handler func(MFAHandlers) http.HandlerFunc) (*mfaHTTPState, *httptest.ResponseRecorder) {
	t.Helper()
	state := &mfaHTTPState{}
	mutate(state)
	handlers, err := NewMFAHandlers(state.config())
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader("code="+code))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler(handlers)(response, request)
	return state, response
}
