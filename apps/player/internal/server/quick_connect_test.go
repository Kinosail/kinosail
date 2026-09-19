package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func quickConnectFixture() servertest.QuickConnectFixture {
	return servertest.QuickConnectFixture{
		New: func(data string, ttl time.Duration) http.Handler {
			return server.New(server.Config{DataDir: data, RequireAuth: true, QuickConnectTTL: ttl})
		},
		SignIn: signInTestProfile, AddViewer: addTestViewer, Web: requestWithCookie, APIKey: apiKeyRequest, API: apiCall,
	}
}

func TestViewerApprovesOneTimeQuickConnectSession(t *testing.T) {
	servertest.AssertViewerApprovesOneTimeQuickConnectSession(t, quickConnectFixture(), true)
}

func TestApprovedQuickConnectCannotBeReassignedToAnotherViewer(t *testing.T) {
	servertest.AssertApprovedQuickConnectCannotBeReassignedToAnotherViewer(t, quickConnectFixture())
}

func TestQuickConnectCodeExpires(t *testing.T) {
	servertest.AssertQuickConnectCodeExpires(t, quickConnectFixture())
}

func TestQuickConnectCreationIsRateLimited(t *testing.T) {
	servertest.AssertQuickConnectCreationIsRateLimited(t, quickConnectFixture())
}

func TestQuickConnectRejectsInvalidDeviceBeforeCreatingRequest(t *testing.T) {
	servertest.AssertQuickConnectRejectsInvalidDeviceBeforeCreatingRequest(t, quickConnectFixture())
}

func TestQuickConnectPageUsesSixDigitAutoSubmitControl(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	page := requestWithCookie(t, handler, http.MethodGet, "/quick-connect", "", owner)
	for _, expected := range []string{`/static/app.css?v=electric-17`, `/static/quick-connect.js?v=1`, `data-quick-connect-digit`, `maxlength="1"`, `inputmode="numeric"`, `pattern="[0-9]"`, `aria-label="Digit 6 of 6"`, "six-digit code"} {
		if !strings.Contains(page.Body.String(), expected) {
			t.Fatalf("Quick Connect page missing %q: %q", expected, page.Body.String())
		}
	}
	script := requestWithCookie(t, handler, http.MethodGet, "/static/quick-connect.js", "", owner)
	if script.Code != http.StatusOK || !strings.Contains(script.Body.String(), `event.clipboardData`) || !strings.Contains(script.Body.String(), `form.requestSubmit()`) || !strings.Contains(script.Body.String(), `value.value.length === inputs.length`) {
		t.Fatalf("Quick Connect script = %d %q", script.Code, script.Body.String())
	}
}

func TestQuickConnectPollRejectsAmbiguousInputWithoutConsumingGrant(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true, QuickConnectTTL: time.Minute})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	for name, test := range map[string]struct {
		path        func(string) string
		contentType string
		body        func(string) string
	}{
		"query": {
			path:        func(string) string { return "/api/v1/quick-connect/token?extra=true" },
			contentType: "application/x-www-form-urlencoded", body: func(secret string) string { return "secret=" + secret },
		},
		"duplicate form": {
			path:        func(string) string { return "/api/v1/quick-connect/token" },
			contentType: "application/x-www-form-urlencoded", body: func(secret string) string { return "secret=" + secret + "&secret=" + secret },
		},
		"unknown form": {
			path:        func(string) string { return "/api/v1/quick-connect/token" },
			contentType: "application/x-www-form-urlencoded", body: func(secret string) string { return "secret=" + secret + "&extra=true" },
		},
		"JSONP media type": {
			path:        func(string) string { return "/api/v1/quick-connect/token" },
			contentType: "application/jsonp", body: func(secret string) string { return `{"secret":"` + secret + `"}` },
		},
		"duplicate JSON": {
			path:        func(string) string { return "/api/v1/quick-connect/token" },
			contentType: "application/json", body: func(secret string) string { return `{"secret":"` + secret + `","secret":"` + secret + `"}` },
		},
		"unknown JSON": {
			path:        func(string) string { return "/api/v1/quick-connect/token" },
			contentType: "application/json", body: func(secret string) string { return `{"secret":"` + secret + `","extra":true}` },
		},
		"oversized JSON": {
			path:        func(string) string { return "/api/v1/quick-connect/token" },
			contentType: "application/json", body: func(string) string { return `{"secret":"` + strings.Repeat("a", 4096) + `"}` },
		},
	} {
		t.Run(name, func(t *testing.T) {
			started := quickConnect(t, handler, "/api/v1/quick-connect", "device=Bedroom+TV")
			approved := requestWithCookie(t, handler, http.MethodPost, "/api/v1/quick-connect/"+started.Code, "", owner)
			if approved.Code != http.StatusNoContent {
				t.Fatalf("approve = %d %q", approved.Code, approved.Body.String())
			}
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, test.path(started.Secret), strings.NewReader(test.body(started.Secret)))
			request.Header.Set("Content-Type", test.contentType)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid poll = %d %q", response.Code, response.Body.String())
			}
			connected := quickConnect(t, handler, "/api/v1/quick-connect/token", "secret="+started.Secret)
			if connected.Token == "" {
				t.Fatalf("invalid poll consumed grant: %+v", connected)
			}
		})
	}
}

func TestQuickConnectPollPreservesValidJSONClients(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true, QuickConnectTTL: time.Minute})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	for _, field := range []string{"secret", "Secret"} {
		started := quickConnect(t, handler, "/api/v1/quick-connect", "device=Bedroom+TV")
		approved := requestWithCookie(t, handler, http.MethodPost, "/api/v1/quick-connect/"+started.Code, "", owner)
		if approved.Code != http.StatusNoContent {
			t.Fatalf("approve = %d %q", approved.Code, approved.Body.String())
		}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect/token", strings.NewReader(`{"`+field+`":"`+started.Secret+`"}`))
		request.Header.Set("Content-Type", "application/json; charset=utf-8")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		var connected quickConnectResponse
		if err := json.Unmarshal(response.Body.Bytes(), &connected); err != nil || response.Code != http.StatusCreated || connected.Token == "" {
			t.Fatalf("valid JSON poll = %d %q", response.Code, response.Body.String())
		}
	}
}

func TestQuickConnectWebApprovalRejectsAmbiguousInputWithoutApproval(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true, QuickConnectTTL: time.Minute})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	for name, test := range map[string]struct {
		path func(string) string
		body func(string) string
	}{
		"query": {
			path: func(code string) string { return "/quick-connect?code=" + code },
			body: func(code string) string { return "code=" + code },
		},
		"duplicate code": {
			path: func(string) string { return "/quick-connect" },
			body: func(code string) string { return "code=" + code + "&code=" + code },
		},
		"unknown field": {
			path: func(string) string { return "/quick-connect" },
			body: func(code string) string { return "code=" + code + "&extra=true" },
		},
	} {
		t.Run(name, func(t *testing.T) {
			started := quickConnect(t, handler, "/api/v1/quick-connect", "device=Bedroom+TV")
			response := requestWithCookie(t, handler, http.MethodPost, test.path(started.Code), test.body(started.Code), owner)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid approval = %d %q", response.Code, response.Body.String())
			}
			pending := quickConnectRecorder(t, handler, "/api/v1/quick-connect/token", "secret="+started.Secret)
			if pending.Code != http.StatusAccepted {
				t.Fatalf("invalid form changed approval = %d %q", pending.Code, pending.Body.String())
			}
			approved := requestWithCookie(t, handler, http.MethodPost, "/quick-connect", "code="+started.Code, owner)
			if approved.Code != http.StatusSeeOther {
				t.Fatalf("valid approval = %d %q", approved.Code, approved.Body.String())
			}
			connected := quickConnect(t, handler, "/api/v1/quick-connect/token", "secret="+started.Secret)
			if connected.Token == "" {
				t.Fatalf("valid approval did not preserve request: %+v", connected)
			}
		})
	}
}

type quickConnectResponse = servertest.QuickConnectResponse

var (
	quickConnect         = servertest.QuickConnect
	quickConnectRecorder = servertest.QuickConnectRecorder
)
