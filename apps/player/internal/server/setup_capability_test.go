package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestSetupCapabilityContract(t *testing.T) {
	servertest.SetupCapability(t, func(dataDir string) http.Handler {
		return server.New(server.Config{DataDir: dataDir, RequireAuth: true})
	})
}

func TestFirstOwnerSetupNeedsNoCode(t *testing.T) {
	t.Parallel()

	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	page := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/setup", nil)
	handler.ServeHTTP(page, request)
	if page.Code != http.StatusOK || strings.Contains(page.Body.String(), `name="setupCode"`) || strings.Contains(page.Body.String(), "Server log") {
		t.Fatalf("setup page = %d %q", page.Code, page.Body.String())
	}

	response := servertest.SetupAPIRequest(t, handler, map[string]any{"name": "Owner", "password": "owner-password", "device": "test"})
	if response.Code != http.StatusCreated {
		t.Fatalf("code-free setup = %d %q", response.Code, response.Body.String())
	}
	if replay := servertest.SetupAPIRequest(t, handler, map[string]any{"name": "Other", "password": "other-password", "device": "test"}); replay.Code != http.StatusConflict {
		t.Fatalf("replayed setup = %d %q", replay.Code, replay.Body.String())
	}
}

func TestFirstOwnerWebSetupNeedsNoCode(t *testing.T) {
	t.Parallel()

	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	allowed := servertest.SetupWebRequest(t, handler, "192.0.2.10:1234", "name=Owner&password=owner-password")
	if allowed.Code != http.StatusSeeOther {
		t.Fatalf("code-free web setup = %d %q", allowed.Code, allowed.Body.String())
	}
}
