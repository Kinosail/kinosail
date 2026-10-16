//go:build !windows

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/commandtest"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestExitingServerHelperProcess(t *testing.T) {
	commandtest.ExitingHelperProcess(t, main)
}

func TestTerminationPersistsAccessUseBeforeExit(t *testing.T) {
	process := commandtest.StartServerProcess(t, "TestExitingServerHelperProcess", "KINOSAIL_MEDIA_DIR="+t.TempDir(), "KINOSAIL_CACHE_DIR="+t.TempDir())
	_, owner := servertest.SetupRouteAuthorization(t, process.Handler, setupToken)
	key := servertest.CreateRouteAPIKey(t, process.Handler, owner, "library")
	servertest.AssertAPIBody(t, servertest.RouteJSON(t, process.Handler, key, http.MethodGet, "/api/v1/library", nil), http.StatusOK)
	process.Terminate(t)
	store, err := database.Open(process.DataDir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var keys map[string]struct {
		LastUsed int64 `json:"lastUsed"`
	}
	if _, err := store.LoadJSON("api_keys.json", &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("persisted API keys = %d", len(keys))
	}
	for _, value := range keys {
		if value.LastUsed == 0 {
			t.Fatal("API key use was not persisted before exit")
		}
	}
}

func setupToken(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var setup struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(response.Body.Bytes(), &setup) != nil || setup.Token == "" {
		t.Fatalf("setup token = %q", response.Body.String())
	}
	return setup.Token
}
