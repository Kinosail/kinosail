package updatecontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

func TestNewHTTPHandlersRejectsMissingAdaptersBeforeEffects(t *testing.T) {
	t.Parallel()
	checker := newHTTPTestChecker(t, nil, nil, nil)
	valid := testHTTPConfig()
	for name, mutate := range map[string]func(*HTTPConfig){
		"reader":    func(config *HTTPConfig) { config.ReadJSON = nil },
		"json":      func(config *HTTPConfig) { config.JSON = nil },
		"api error": func(config *HTTPConfig) { config.APIError = nil },
		"web error": func(config *HTTPConfig) { config.WebError = nil },
	} {
		t.Run(name, func(t *testing.T) {
			config := valid
			mutate(&config)
			if handlers, err := NewHTTPHandlers(checker, config); err == nil || handlers != nil {
				t.Fatalf("invalid config created handlers %#v, err=%v", handlers, err)
			}
		})
	}
	if handlers, err := NewHTTPHandlers(nil, valid); err == nil || handlers != nil {
		t.Fatalf("nil checker created handlers %#v, err=%v", handlers, err)
	}
}

func TestUpdatePreferenceRejectsInvalidJSONBeforePersistence(t *testing.T) {
	t.Parallel()
	var saves atomic.Int32
	checker := newHTTPTestChecker(t, nil, func(bool) error { saves.Add(1); return nil }, nil)
	handlers := mustHTTPHandlers(t, checker)
	for _, body := range []string{`{}`, `{"automatic":null}`, `{"automatic":true,"extra":false}`, `{`, `{"automatic":true}{}`} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/settings/updates", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handlers.Preference(response, request)
		if response.Code != http.StatusBadRequest || saves.Load() != 0 {
			t.Fatalf("body %q changed state: code=%d saves=%d", body, response.Code, saves.Load())
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/settings/updates", strings.NewReader(`{"automatic":false}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handlers.Preference(response, request)
	if response.Code != http.StatusOK || saves.Load() != 1 {
		t.Fatalf("valid preference: code=%d saves=%d body=%q", response.Code, saves.Load(), response.Body.String())
	}
}

func TestUpdatePreferencePersistenceFailureDoesNotTriggerCheck(t *testing.T) {
	t.Parallel()
	var sourceCalls atomic.Int32
	checker := newHTTPTestChecker(t, &sourceCalls, func(bool) error { return errors.New("save failed") }, nil)
	handlers := mustHTTPHandlers(t, checker)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/settings/updates", strings.NewReader(`{"automatic":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handlers.Preference(response, request)
	if response.Code != http.StatusInternalServerError || sourceCalls.Load() != 0 || len(checker.trigger) != 0 {
		t.Fatalf("failed save: code=%d calls=%d triggers=%d", response.Code, sourceCalls.Load(), len(checker.trigger))
	}
}

func TestManualCheckRejectsInvalidJSONBeforeNetworkAccess(t *testing.T) {
	t.Parallel()
	var sourceCalls atomic.Int32
	checker := newHTTPTestChecker(t, &sourceCalls, nil, nil)
	handlers := mustHTTPHandlers(t, checker)
	for _, body := range []string{"", `{"unexpected":true}`, `{`, `{} {}`} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/updates/check", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handlers.Check(response, request)
		if response.Code != http.StatusBadRequest || sourceCalls.Load() != 0 || checker.View().State != "not-checked" {
			t.Fatalf("body %q: code=%d calls=%d status=%#v", body, response.Code, sourceCalls.Load(), checker.View())
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/updates/check", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handlers.Check(response, request)
	if response.Code != http.StatusOK || sourceCalls.Load() != 1 {
		t.Fatalf("valid check: code=%d calls=%d body=%q", response.Code, sourceCalls.Load(), response.Body.String())
	}
}

func TestManualCheckMapsReleaseFailureToServiceUnavailable(t *testing.T) {
	t.Parallel()
	checker := newHTTPTestChecker(t, nil, nil, errors.New("offline"))
	handlers := mustHTTPHandlers(t, checker)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/updates/check", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handlers.Check(response, request)
	if response.Code != http.StatusServiceUnavailable || checker.View().State != "unavailable" {
		t.Fatalf("failed check: code=%d status=%#v", response.Code, checker.View())
	}
}

func TestInstallRequestValidatesEmptyInputBeforeManagerMutation(t *testing.T) { //nolint:cyclop // All rejected transport shapes share the manager side-effect assertion.
	t.Parallel()
	manager, err := New(nil, PlayerPolicy(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	manager.random = bytes.NewReader(bytes.Repeat([]byte{5}, 32))
	checker := newHTTPTestChecker(t, nil, nil, nil)
	checker.manager = manager
	checker.source = ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) { return "v1.1.0", "", false, nil })
	checker.Check(t.Context())
	handlers := mustHTTPHandlers(t, checker)
	for name, request := range map[string]*http.Request{
		"body":  httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/updates", strings.NewReader(`{}`)),
		"query": httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/updates?next=true", nil),
		"form":  formRequest(t, "/api/v1/updates", "extra=true"),
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handlers.Request(response, request)
			view, viewErr := manager.View("v1.0.0")
			if response.Code != http.StatusBadRequest || viewErr != nil || view.RequestID != "" {
				t.Fatalf("invalid request: code=%d view=%#v err=%v", response.Code, view, viewErr)
			}
		})
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/updates", nil)
	response := httptest.NewRecorder()
	handlers.Request(response, request)
	view, viewErr := manager.View("v1.0.0")
	if response.Code != http.StatusAccepted || viewErr != nil || view.RequestID == "" || view.TargetVersion != "v1.1.0" || !strings.Contains(response.Body.String(), `"targetVersion":"v1.1.0"`) {
		t.Fatalf("valid request: code=%d view=%#v err=%v body=%q", response.Code, view, viewErr, response.Body.String())
	}
}

func TestInstallRequestRequiresAvailableReleaseAndWritableManager(t *testing.T) {
	t.Parallel()
	checker := newHTTPTestChecker(t, nil, nil, nil)
	handlers := mustHTTPHandlers(t, checker)
	response := httptest.NewRecorder()
	handlers.Request(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/updates", nil))
	if response.Code != http.StatusConflict {
		t.Fatalf("unchecked request = %d %q", response.Code, response.Body.String())
	}
	manager, _ := New(nil, PlayerPolicy(1, 1))
	manager.random = errorReader{}
	checker.manager = manager
	checker.status = Status{State: "available", CurrentVersion: "v1.0.0", LatestVersion: "v1.1.0", ReleaseURL: GitHubReleasesURL}
	response = httptest.NewRecorder()
	handlers.Request(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/updates", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("manager failure = %d %q", response.Code, response.Body.String())
	}
}

func TestSavePreferenceRejectsInvalidFormsBeforePersistence(t *testing.T) { //nolint:cyclop // All strict form cases share the persistence assertion.
	t.Parallel()
	var saves atomic.Int32
	checker := newHTTPTestChecker(t, nil, func(bool) error { saves.Add(1); return nil }, nil)
	handler := mustHTTPHandlers(t, checker).SavePreference("/settings#updates")
	cases := map[string]*http.Request{
		"content type": httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/updates", strings.NewReader("mode=manual")),
		"query":        formRequest(t, "/settings/updates?extra=true", "mode=manual"),
		"malformed":    formRequest(t, "/settings/updates", "mode=%zz"),
		"missing":      formRequest(t, "/settings/updates", ""),
		"extra":        formRequest(t, "/settings/updates", "mode=manual&extra=true"),
		"duplicate":    formRequest(t, "/settings/updates", "mode=manual&mode=automatic"),
		"unknown":      formRequest(t, "/settings/updates", "mode=sometimes"),
		"oversized":    formRequest(t, "/settings/updates", "mode="+strings.Repeat("x", 10)),
	}
	for name, request := range cases {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler(response, request)
			if response.Code != http.StatusBadRequest || saves.Load() != 0 {
				t.Fatalf("invalid form: code=%d saves=%d", response.Code, saves.Load())
			}
		})
	}
	response := httptest.NewRecorder()
	handler(response, formRequest(t, "/settings/updates", "mode=automatic"))
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/settings#updates" || saves.Load() != 1 {
		t.Fatalf("valid form: code=%d location=%q saves=%d", response.Code, response.Header().Get("Location"), saves.Load())
	}
}

func TestCheckAndRequestRejectsInvalidFormsBeforeNetworkAccess(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	checker := newHTTPTestChecker(t, &calls, nil, nil)
	handler := mustHTTPHandlers(t, checker).CheckAndRequest("/settings#updates")
	for _, request := range []*http.Request{
		httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/updates/check", nil),
		formRequest(t, "/settings/updates/check?extra=true", ""),
		formRequest(t, "/settings/updates/check", "extra=true"),
		formRequest(t, "/settings/updates/check", "%zz"),
	} {
		response := httptest.NewRecorder()
		handler(response, request)
		if response.Code != http.StatusBadRequest || calls.Load() != 0 {
			t.Fatalf("invalid check: code=%d calls=%d", response.Code, calls.Load())
		}
	}
	response := httptest.NewRecorder()
	handler(response, formRequest(t, "/settings/updates/check", ""))
	if response.Code != http.StatusSeeOther || calls.Load() != 1 {
		t.Fatalf("valid check: code=%d calls=%d", response.Code, calls.Load())
	}
}

func newHTTPTestChecker(t *testing.T, calls *atomic.Int32, save func(bool) error, sourceErr error) *Checker {
	t.Helper()
	if save == nil {
		save = func(bool) error { return nil }
	}
	return mustChecker(t, CheckerConfig{
		CurrentVersion: "v1.0.0", Automatic: func() bool { return false }, SaveAutomatic: save,
		Source: ReleaseSourceFunc(func(context.Context, string) (string, string, bool, error) {
			if calls != nil {
				calls.Add(1)
			}
			return "v1.0.0", "", false, sourceErr
		}),
	})
}

func mustHTTPHandlers(t *testing.T, checker *Checker) *HTTPHandlers {
	t.Helper()
	handlers, err := NewHTTPHandlers(checker, testHTTPConfig())
	if err != nil {
		t.Fatal(err)
	}
	return handlers
}

func testHTTPConfig() HTTPConfig {
	jsonResponse := func(writer http.ResponseWriter, value any, status int) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(value)
	}
	apiError := func(writer http.ResponseWriter, err error, status int) {
		jsonResponse(writer, map[string]string{"error": err.Error()}, status)
	}
	return HTTPConfig{
		ReadJSON: func(writer http.ResponseWriter, request *http.Request, target any) bool {
			if err := httpguard.DecodeRequestJSON(writer, request, target); err != nil {
				apiError(writer, err, http.StatusBadRequest)
				return false
			}
			return true
		},
		JSON: jsonResponse, APIError: apiError,
		WebError: func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
			http.Error(writer, message, status)
		},
	}
}

func formRequest(t *testing.T, target, body string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}
