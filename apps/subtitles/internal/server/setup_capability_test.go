package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestSetupCapabilityContract(t *testing.T) {
	servertest.SetupCapability(t, func(dataDir string) http.Handler {
		return server.New(server.Config{DataDir: dataDir, RequireAuth: true})
	})
}

func TestFirstOwnerSetupNeedsNoServerCode(t *testing.T) {
	t.Parallel()

	web := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	page := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/setup", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	web.ServeHTTP(page, request)
	if page.Code != http.StatusOK || strings.Contains(page.Body.String(), "Setup code") || strings.Contains(page.Body.String(), "setup_code") {
		t.Fatalf("setup page = %d %q", page.Code, page.Body.String())
	}
	if response := servertest.SetupWebRequest(t, web, "192.0.2.10:1234", "name=Owner&password=owner-password"); response.Code != http.StatusSeeOther {
		t.Fatalf("web setup = %d %q", response.Code, response.Body.String())
	}

	api := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	response := servertest.SetupAPIRequest(t, api, map[string]any{"name": "Owner", "password": "owner-password", "device": "test"})
	if response.Code != http.StatusCreated {
		t.Fatalf("API setup = %d %q", response.Code, response.Body.String())
	}
}
