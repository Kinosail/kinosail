package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-dashboard/internal/auth"
	"github.com/MikeO7/kinosail-dashboard/internal/dashboard"
	"github.com/MikeO7/kinosail-dashboard/internal/database"
)

type testApplication struct {
	handler http.Handler
	board   *dashboard.Service
	auth    *auth.Manager
	prober  *dashboard.Prober
}

type testSession struct {
	cookie *http.Cookie
	csrf   string
}

func newTestApplication(t *testing.T, config Config) testApplication {
	t.Helper()
	config.TrustedHosts = append(config.TrustedHosts, "example.com")
	store, err := database.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	authentication, err := auth.NewManager(context.Background(), store)
	if err != nil {
		t.Fatal(err)
	}
	board, err := dashboard.NewService(context.Background(), store)
	if err != nil {
		t.Fatal(err)
	}
	prober := dashboard.NewProber(board, time.Hour, nil)
	return testApplication{
		handler: New(config, board, authentication, prober),
		board:   board,
		auth:    authentication,
		prober:  prober,
	}
}

func (app testApplication) request(t *testing.T, method, path, body string, session *testSession) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if session != nil {
		request.AddCookie(session.cookie)
		if session.csrf != "" {
			request.Header.Set("X-Kinosail-CSRF", session.csrf)
		}
	}
	response := httptest.NewRecorder()
	app.handler.ServeHTTP(response, request)
	return response
}

func (app testApplication) setup(t *testing.T) testSession {
	t.Helper()
	response := app.request(t, http.MethodPost, "/api/v1/setup", `{"name":"Owner","password":"long-password-123","device":"test browser"}`, nil)
	if response.Code != http.StatusCreated {
		t.Fatalf("setup status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Kinosail-Login-Next") != "/?passkey=offer" {
		t.Fatalf("setup next = %q", response.Header().Get("X-Kinosail-Login-Next"))
	}
	var output struct {
		CSRF string `json:"csrf"`
	}
	decodeBody(t, response.Body, &output)
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != auth.SessionCookie {
		t.Fatalf("setup cookies = %#v", cookies)
	}
	return testSession{cookie: cookies[0], csrf: output.CSRF}
}

func requireJSONError(t *testing.T, response *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, status, response.Body.String())
	}
	var output map[string]string
	decodeBody(t, response.Body, &output)
	if output["error"] != message {
		t.Fatalf("error = %q, want %q", output["error"], message)
	}
}

func decodeBody(t *testing.T, body io.Reader, target any) {
	t.Helper()
	if err := json.NewDecoder(body).Decode(target); err != nil {
		t.Fatal(err)
	}
}
