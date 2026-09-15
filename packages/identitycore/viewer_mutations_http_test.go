package identitycore

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type viewerMutationEffects struct {
	profiles, sessions, keys int
	id, password             string
	err                      error
}

func (effects *viewerMutationEffects) reset(id, password string) error {
	effects.profiles++
	effects.sessions++
	effects.keys++
	effects.id, effects.password = id, password
	return effects.err
}

func (effects *viewerMutationEffects) remove(id string) error {
	effects.profiles++
	effects.sessions++
	effects.keys++
	effects.id = id
	return effects.err
}

func (effects *viewerMutationEffects) unchanged() bool {
	return effects.profiles == 0 && effects.sessions == 0 && effects.keys == 0
}

type viewerMutationError struct {
	message string
	status  int
}

func (result *viewerMutationError) write(writer http.ResponseWriter, _ *http.Request, message string, status int) {
	result.message, result.status = message, status
	http.Error(writer, message, status)
}

func viewerMutationRequest(t *testing.T, path, body, contentType string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return request
}

func TestViewerPasswordResetHandlerRejectsInvalidFormsWithoutSideEffects(t *testing.T) {
	t.Parallel()
	password := "windward-viewer-2026"
	tests := map[string]struct {
		path, body, contentType string
	}{
		"missing media type":   {"/settings/profiles/password", "id=viewer&password=" + password, ""},
		"wrong media type":     {"/settings/profiles/password", "id=viewer&password=" + password, "text/plain"},
		"query":                {"/settings/profiles/password?id=query", "id=viewer&password=" + password, "application/x-www-form-urlencoded"},
		"malformed":            {"/settings/profiles/password", "id=%zz&password=" + password, "application/x-www-form-urlencoded"},
		"unknown field":        {"/settings/profiles/password", "id=viewer&password=" + password + "&owner=true", "application/x-www-form-urlencoded"},
		"repeated id":          {"/settings/profiles/password", "id=viewer&id=viewer&password=" + password, "application/x-www-form-urlencoded"},
		"conflicting id":       {"/settings/profiles/password", "id=viewer&id=other&password=" + password, "application/x-www-form-urlencoded"},
		"conflicting password": {"/settings/profiles/password", "id=viewer&password=first-password&password=second-password", "application/x-www-form-urlencoded"},
		"missing id":           {"/settings/profiles/password", "password=" + password, "application/x-www-form-urlencoded"},
		"empty id":             {"/settings/profiles/password", "id=&password=" + password, "application/x-www-form-urlencoded"},
		"oversized id":         {"/settings/profiles/password", "id=" + strings.Repeat("i", maxIdentityLength+1) + "&password=" + password, "application/x-www-form-urlencoded"},
		"missing password":     {"/settings/profiles/password", "id=viewer", "application/x-www-form-urlencoded"},
		"empty password":       {"/settings/profiles/password", "id=viewer&password=", "application/x-www-form-urlencoded"},
		"oversized password":   {"/settings/profiles/password", "id=viewer&password=" + strings.Repeat("p", viewerPasswordLimit+1), "application/x-www-form-urlencoded"},
		"oversized body":       {"/settings/profiles/password", "id=viewer&password=" + strings.Repeat("p", viewerMutationFormLimit), "application/x-www-form-urlencoded"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			effects, result := new(viewerMutationEffects), new(viewerMutationError)
			response := httptest.NewRecorder()
			ViewerPasswordResetHandler(effects.reset, result.write).ServeHTTP(response, viewerMutationRequest(t, test.path, test.body, test.contentType))
			if response.Code != http.StatusBadRequest || result.status != http.StatusBadRequest || result.message != invalidViewerProfile || !effects.unchanged() {
				t.Fatalf("response = %d, error = %#v, effects = %#v", response.Code, result, effects)
			}
		})
	}
}

func TestViewerRemovalHandlerRejectsInvalidFormsWithoutSideEffects(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"missing id":     "",
		"empty id":       "id=",
		"unknown field":  "id=viewer&owner=true",
		"repeated id":    "id=viewer&id=viewer",
		"conflicting id": "id=viewer&id=other",
		"oversized id":   "id=" + strings.Repeat("i", maxIdentityLength+1),
		"oversized body": "id=" + strings.Repeat("i", viewerMutationFormLimit),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			effects, result := new(viewerMutationEffects), new(viewerMutationError)
			response := httptest.NewRecorder()
			ViewerRemovalHandler(effects.remove, result.write).ServeHTTP(response, viewerMutationRequest(t, "/settings/profiles/remove", body, "application/x-www-form-urlencoded"))
			if response.Code != http.StatusBadRequest || result.status != http.StatusBadRequest || result.message != invalidViewerProfile || !effects.unchanged() {
				t.Fatalf("response = %d, error = %#v, effects = %#v", response.Code, result, effects)
			}
		})
	}
}

func TestViewerMutationHandlersAcceptExactFieldLimits(t *testing.T) {
	t.Parallel()
	id, password := strings.Repeat("i", maxIdentityLength), strings.Repeat("p", viewerPasswordLimit)
	assertViewerResetAtLimits(t, id, password)
	assertViewerRemovalAtLimits(t, id)
}

func assertViewerResetAtLimits(t *testing.T, id, password string) {
	t.Helper()
	resetEffects := new(viewerMutationEffects)
	resetResponse := httptest.NewRecorder()
	resetBody := url.Values{"id": {id}, "password": {password}}.Encode()
	ViewerPasswordResetHandler(resetEffects.reset, new(viewerMutationError).write).ServeHTTP(resetResponse, viewerMutationRequest(t, "/settings/profiles/password", resetBody, "application/x-www-form-urlencoded; charset=utf-8"))
	if resetResponse.Code != http.StatusSeeOther || resetResponse.Header().Get("Location") != "/settings" || resetEffects.id != id || resetEffects.password != password || resetEffects.profiles != 1 || resetEffects.sessions != 1 || resetEffects.keys != 1 {
		t.Fatalf("reset response = %d %q, effects = %#v", resetResponse.Code, resetResponse.Header().Get("Location"), resetEffects)
	}
}

func assertViewerRemovalAtLimits(t *testing.T, id string) {
	t.Helper()
	removeEffects := new(viewerMutationEffects)
	removeResponse := httptest.NewRecorder()
	ViewerRemovalHandler(removeEffects.remove, new(viewerMutationError).write).ServeHTTP(removeResponse, viewerMutationRequest(t, "/settings/profiles/remove", url.Values{"id": {id}}.Encode(), "application/x-www-form-urlencoded"))
	if removeResponse.Code != http.StatusSeeOther || removeResponse.Header().Get("Location") != "/settings" || removeEffects.id != id || removeEffects.profiles != 1 || removeEffects.sessions != 1 || removeEffects.keys != 1 {
		t.Fatalf("remove response = %d %q, effects = %#v", removeResponse.Code, removeResponse.Header().Get("Location"), removeEffects)
	}
}

func TestViewerMutationHandlersPreserveDomainErrors(t *testing.T) {
	t.Parallel()
	storeError := errors.New("profile transaction failed")
	for name, handler := range map[string]func(*viewerMutationEffects, *viewerMutationError) http.HandlerFunc{
		"reset": func(effects *viewerMutationEffects, result *viewerMutationError) http.HandlerFunc {
			return ViewerPasswordResetHandler(effects.reset, result.write)
		},
		"remove": func(effects *viewerMutationEffects, result *viewerMutationError) http.HandlerFunc {
			return ViewerRemovalHandler(effects.remove, result.write)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			effects, result := &viewerMutationEffects{err: storeError}, new(viewerMutationError)
			response := httptest.NewRecorder()
			body := "id=viewer"
			if name == "reset" {
				body += "&password=windward-viewer-2026"
			}
			handler(effects, result).ServeHTTP(response, viewerMutationRequest(t, "/settings/profiles/"+name, body, "application/x-www-form-urlencoded"))
			if response.Code != http.StatusBadRequest || result.status != http.StatusBadRequest || result.message != storeError.Error() || effects.profiles != 1 || effects.sessions != 1 || effects.keys != 1 || response.Header().Get("Location") != "" {
				t.Fatalf("response = %d %q, error = %#v, effects = %#v", response.Code, response.Header().Get("Location"), result, effects)
			}
		})
	}
}

func TestViewerMutationHandlersFailClosedWithoutCallbacks(t *testing.T) {
	t.Parallel()
	tests := map[string]http.HandlerFunc{
		"reset mutation":  ViewerPasswordResetHandler(nil, new(viewerMutationError).write),
		"reset error":     ViewerPasswordResetHandler(func(string, string) error { return nil }, nil),
		"remove mutation": ViewerRemovalHandler(nil, new(viewerMutationError).write),
		"remove error":    ViewerRemovalHandler(func(string) error { return nil }, nil),
	}
	for name, handler := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, viewerMutationRequest(t, "/settings", "", "application/x-www-form-urlencoded"))
			if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "mutation is unavailable") {
				t.Fatalf("response = %d %q", response.Code, response.Body.String())
			}
		})
	}
}
