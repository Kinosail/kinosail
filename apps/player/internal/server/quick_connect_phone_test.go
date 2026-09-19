package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestPhoneApprovalRequiresSessionAndExplicitAction(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true, QuickConnectTTL: time.Minute})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	started := quickConnect(t, handler, "/api/v1/quick-connect", "device=Kinosail+TV")
	assertPhonePreviewAuthorization(t, handler, owner, started.Code, started.Secret)
	assertPhoneLandingRecovery(t, handler, started.Code, started.Secret)
	redirect := httptest.NewRecorder()
	handler.ServeHTTP(redirect, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/quick-connect?code="+started.Code, nil))
	if redirect.Header().Get("Location") != "/login?next="+url.QueryEscape("/quick-connect?code="+started.Code) {
		t.Fatalf("lost scanned code at login: %q", redirect.Header().Get("Location"))
	}
	confirm := requestWithCookie(t, handler, http.MethodGet, "/quick-connect?code="+started.Code, "", owner)
	if confirm.Code != http.StatusOK || !strings.Contains(confirm.Body.String(), "Approve device") || strings.Contains(confirm.Body.String(), "data-quick-connect-digit") {
		t.Fatalf("confirmation = %d %q", confirm.Code, confirm.Body.String())
	}
	pending := quickConnectRecorder(t, handler, "/api/v1/quick-connect/token", "secret="+started.Secret)
	if pending.Code != http.StatusAccepted {
		t.Fatal("reading approval screens approved the TV")
	}
	approved := requestWithCookie(t, handler, http.MethodPost, "/api/v1/quick-connect/"+started.Code, "", owner)
	if approved.Code != http.StatusNoContent {
		t.Fatalf("approval = %d", approved.Code)
	}
	connected := quickConnect(t, handler, "/api/v1/quick-connect/token", "secret="+started.Secret)
	if connected.Token == "" {
		t.Fatal("TV did not receive its own session")
	}
	replayed := requestWithCookie(t, handler, http.MethodPost, "/api/v1/quick-connect/"+started.Code, "", owner)
	if replayed.Code != http.StatusBadRequest {
		t.Fatalf("consumed code replay = %d", replayed.Code)
	}
}

func TestPhoneApprovalIsUnavailableOnPublicListener(t *testing.T) {
	t.Parallel()
	handler := server.Remote(server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true}))
	for _, path := range []string{"/connect?code=123456", "/quick-connect?code=123456", "/api/v1/quick-connect/pending", "/api/v1/quick-connect/123456"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("public %s = %d", path, response.Code)
		}
	}
}

func TestPhoneApprovalCancelRejectsMalformedInputWithoutSideEffects(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir()})
	started := quickConnect(t, handler, "/api/v1/quick-connect", "device=Kinosail+TV")
	for _, body := range []string{"", "secret=", "secret=x&secret=y", "secret=x&other=y", "secret=" + strings.Repeat("x", 5000)} {
		response := quickConnectRecorder(t, handler, "/api/v1/quick-connect/cancel", body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid cancel = %d", response.Code)
		}
		if response := quickConnectRecorder(t, handler, "/api/v1/quick-connect/token", "secret="+started.Secret); response.Code != http.StatusAccepted {
			t.Fatal("invalid cancellation removed the request")
		}
	}
	cancelled := quickConnectRecorder(t, handler, "/api/v1/quick-connect/cancel", "secret="+started.Secret)
	if cancelled.Code != http.StatusNoContent {
		t.Fatalf("cancel = %d", cancelled.Code)
	}
	if response := quickConnectRecorder(t, handler, "/api/v1/quick-connect/token", "secret="+started.Secret); response.Code != http.StatusNotFound {
		t.Fatal("cancelled request remained live")
	}
}

func assertPhonePreviewAuthorization(t *testing.T, handler http.Handler, owner *http.Cookie, code, secret string) {
	t.Helper()
	for _, path := range []string{"/api/v1/quick-connect/pending", "/api/v1/quick-connect/" + code} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous preview = %d", response.Code)
		}
		response = requestWithCookie(t, handler, http.MethodGet, path, "", owner)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), code) || strings.Contains(response.Body.String(), secret) {
			t.Fatalf("preview = %d %q", response.Code, response.Body.String())
		}
	}
}

func assertPhoneLandingRecovery(t *testing.T, handler http.Handler, code, secret string) {
	t.Helper()
	landing := httptest.NewRecorder()
	handler.ServeHTTP(landing, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/connect?code="+code, nil))
	if landing.Code != http.StatusOK || !strings.Contains(landing.Body.String(), "Open Player app") || !strings.Contains(landing.Body.String(), "Use browser") {
		t.Fatalf("landing = %d %q", landing.Code, landing.Body.String())
	}
	if strings.Contains(landing.Body.String(), secret) || landing.Header().Get("Cache-Control") != "no-store" || landing.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("unsafe landing")
	}
}

func TestPhoneScannerCameraPolicyIsLimitedToPlayerConnectPage(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	for _, path := range []string{"/quick-connect", "/account"} {
		response := requestWithCookie(t, handler, http.MethodGet, path, "", owner)
		want := "camera=(), microphone=(), geolocation=()"
		if path == "/quick-connect" {
			want = "camera=(self), microphone=(), geolocation=()"
			if !strings.Contains(response.Body.String(), "Scan QR code") || !strings.Contains(response.Body.String(), "data-quick-connect-digit") {
				t.Fatal("scanner or manual entry is missing")
			}
		}
		if response.Header().Get("Permissions-Policy") != want {
			t.Fatalf("%s camera policy = %q", path, response.Header().Get("Permissions-Policy"))
		}
	}
}
