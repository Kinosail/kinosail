package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestLibraryHistoryBrowserJourney(t *testing.T) {
	if os.Getenv("KINOSAIL_LIBRARY_BROWSER") != "1" {
		t.Skip("KINOSAIL_LIBRARY_BROWSER is not set")
	}
	media := t.TempDir()
	for number := 1; number <= 9; number++ {
		if err := os.WriteFile(filepath.Join(media, fmt.Sprintf("History Movie %02d.mp4", number)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	web := httptest.NewServer(server.New(server.Config{Lifecycle: t.Context(), MediaDir: media, DataDir: t.TempDir()}))
	t.Cleanup(web.Close)
	runHistoryBrowserPhase(t, web.URL, "empty")
	listing, err := http.NewRequestWithContext(t.Context(), http.MethodGet, web.URL+"/api/v1/library?view=movies&limit=200", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := web.Client().Do(listing)
	if err != nil {
		t.Fatal(err)
	}
	var library struct {
		Items []struct{ ID, Title string } `json:"items"`
	}
	err = json.NewDecoder(response.Body).Decode(&library)
	_ = response.Body.Close()
	if err != nil || len(library.Items) != 9 {
		t.Fatalf("history fixture library = %d items, %v", len(library.Items), err)
	}
	for _, item := range library.Items {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodPut, web.URL+"/api/v1/items/"+item.ID+"/progress", strings.NewReader(`{"seconds":30}`))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		saved, err := web.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = saved.Body.Close()
		if saved.StatusCode != http.StatusOK {
			t.Fatalf("save progress for %s = %d", item.Title, saved.StatusCode)
		}
		time.Sleep(20 * time.Millisecond)
	}
	runHistoryBrowserPhase(t, web.URL, "loaded")
}

func runHistoryBrowserPhase(t *testing.T, origin, phase string) {
	t.Helper()
	project := os.Getenv("KINOSAIL_BROWSER_PROJECT")
	if project == "" {
		project = "chromium"
	}
	command := exec.CommandContext(t.Context(), "pnpm", "exec", "playwright", "test", "library-history.spec.ts", "--workers=1", "--project="+project) //nolint:gosec // Fixed test command and disposable local Server.
	command.Dir = "../../e2e"
	command.Env = append(os.Environ(), "KINOSAIL_HISTORY_BROWSER_URL="+origin, "KINOSAIL_E2E_URL="+origin, "KINOSAIL_HISTORY_PHASE="+phase)
	if output := os.Getenv("KINOSAIL_E2E_OUTPUT_DIR"); output != "" {
		command.Env = append(command.Env, "KINOSAIL_E2E_OUTPUT_DIR="+filepath.Join(output, "history-"+phase))
	}
	if artifacts := os.Getenv("KINOSAIL_E2E_ARTIFACT_DIR"); artifacts != "" {
		command.Env = append(command.Env, "KINOSAIL_E2E_ARTIFACT_DIR="+filepath.Join(artifacts, "history-"+phase))
	}
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		t.Fatalf("%s history browser phase: %v", phase, err)
	}
}
