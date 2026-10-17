//go:build !windows

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/commandtest"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestExitingServerHelperProcess(t *testing.T) {
	commandtest.ExitingHelperProcess(t, main)
}

func TestTerminationPersistsCoalescedWritesBeforeExit(t *testing.T) {
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Movie.mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	process := commandtest.StartServerProcess(t, "TestExitingServerHelperProcess", "KINOSAIL_MEDIA_DIR="+media, "KINOSAIL_CACHE_DIR="+t.TempDir())
	_, owner := servertest.SetupRouteAuthorization(t, process.Handler, setupToken)
	key := servertest.CreateRouteAPIKey(t, process.Handler, owner, "library")
	item := firstLibraryItem(t, process.Handler, key)
	for _, seconds := range []int{10, 20, 30} {
		servertest.AssertAPIBody(t, servertest.RouteJSON(t, process.Handler, owner, http.MethodPut, "/api/v1/items/"+item+"/progress", map[string]int{"seconds": seconds}), http.StatusOK)
	}
	process.Terminate(t)
	store, err := database.Open(process.DataDir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var progress map[string]struct {
		Seconds float64 `json:"seconds"`
	}
	var keys map[string]struct {
		LastUsed int64 `json:"lastUsed"`
	}
	if _, err := store.LoadJSON("progress.json", &progress); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadJSON("api_keys.json", &keys); err != nil {
		t.Fatal(err)
	}
	persisted := 0.0
	for stored, value := range progress {
		if strings.HasSuffix(stored, ":"+item) {
			persisted = value.Seconds
		}
	}
	if persisted != 30 {
		t.Errorf("persisted progress after SIGTERM = %v, want 30", persisted)
	}
	if len(keys) != 1 {
		t.Fatalf("persisted API keys = %d", len(keys))
	}
	for _, value := range keys {
		if value.LastUsed == 0 {
			t.Error("API key use was not persisted before exit")
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

func firstLibraryItem(t *testing.T, handler http.Handler, key string) string {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		var library struct {
			Items []struct{ ID string }
		}
		response := servertest.RouteJSON(t, handler, key, http.MethodGet, "/api/v1/library", nil)
		if response.Code == http.StatusOK && json.Unmarshal(response.Body.Bytes(), &library) == nil && len(library.Items) > 0 {
			return library.Items[0].ID
		}
	}
	t.Fatal("library has no items")
	return ""
}
