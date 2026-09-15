package apihttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type saveInput struct {
	Name string `json:"name"`
}

func TestSaveReturnsPlayerContract(t *testing.T) {
	conflict := errors.New("managed setting")
	for _, test := range []struct {
		name, body, response string
		err                  error
		status               int
		called               bool
	}{
		{"saved", `{"name":"Player"}`, `{"status":"saved"}` + "\n", nil, http.StatusOK, true},
		{"invalid", `{"name":"Player"}`, `{"error":"invalid setting"}` + "\n", errors.New("invalid setting"), http.StatusBadRequest, true},
		{"conflict", `{"name":"Player"}`, `{"error":"wrapped: managed setting"}` + "\n", fmt.Errorf("wrapped: %w", conflict), http.StatusConflict, true},
		{"malformed", `{`, `{"error":"invalid JSON request"}` + "\n", nil, http.StatusBadRequest, false},
		{"unknown", `{"unknown":true}`, `{"error":"invalid JSON request"}` + "\n", nil, http.StatusBadRequest, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			handler := Save(func(input saveInput) error {
				called = true
				if input.Name != "Player" {
					t.Fatalf("input = %#v", input)
				}
				return test.err
			}, conflict)
			response := httptest.NewRecorder()
			handler(response, httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/settings", strings.NewReader(test.body)))
			if response.Code != test.status || response.Body.String() != test.response || called != test.called {
				t.Fatalf("response = %d %q; called = %t", response.Code, response.Body.String(), called)
			}
			if response.Header().Get("Content-Type") != "application/json" || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("headers = %#v", response.Header())
			}
		})
	}
}

func TestSaveFailsClosedWithoutCallback(t *testing.T) {
	response := httptest.NewRecorder()
	Save[saveInput](nil)(response, httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/settings", strings.NewReader(`{"name":"Player"}`)))
	if response.Code != http.StatusInternalServerError || response.Body.String() != `{"error":"save unavailable"}`+"\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	assertPrivateJSON(t, response)
}

func TestSaveLibrariesReturnsRefreshedList(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/libraries", strings.NewReader(`{"path":"/media"}`))
	response := httptest.NewRecorder()
	changed := ""
	SaveLibraries(func(path string) error {
		changed = path
		return nil
	}, func(ctx context.Context) (any, error) {
		if ctx != request.Context() || changed != "/media" {
			t.Fatalf("refresh context or order is invalid")
		}
		return []string{"/media"}, nil
	})(response, request)
	if response.Code != http.StatusOK || response.Body.String() != `{"libraries":["/media"]}`+"\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestSaveLibrariesRejectsBeforeRefresh(t *testing.T) {
	for _, test := range []struct {
		name, body string
		change     func(string) error
		status     int
	}{
		{"malformed", `{`, func(string) error { t.Fatal("change called"); return nil }, http.StatusBadRequest},
		{"unknown", `{"path":"/media","extra":true}`, func(string) error { t.Fatal("change called"); return nil }, http.StatusBadRequest},
		{"change", `{"path":"/media"}`, func(string) error { return errors.New("invalid library") }, http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			SaveLibraries(test.change, func(context.Context) (any, error) {
				t.Fatal("refresh called")
				return nil, nil
			})(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/libraries", strings.NewReader(test.body)))
			if response.Code != test.status {
				t.Fatalf("status = %d; body = %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestSaveLibrariesHandlesRefreshAndConfigurationFailures(t *testing.T) {
	validRequest := func() *http.Request {
		return httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/libraries", strings.NewReader(`{"path":"/media"}`))
	}
	for _, handler := range []http.HandlerFunc{
		SaveLibraries(nil, func(context.Context) (any, error) { return nil, nil }),
		SaveLibraries(func(string) error { return nil }, nil),
		SaveLibraries(func(string) error { return nil }, func(context.Context) (any, error) { return nil, errors.New("refresh failed") }),
	} {
		response := httptest.NewRecorder()
		handler(response, validRequest())
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; body = %q", response.Code, response.Body.String())
		}
	}
}
