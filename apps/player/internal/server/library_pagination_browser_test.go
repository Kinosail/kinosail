package server_test

import (
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

// This opt-in journey keeps the real Server alive while Playwright browses its
// disposable catalog. Fault interception is documented separately in the spec.
func TestLibraryPaginationBrowserJourney(t *testing.T) {
	if os.Getenv("KINOSAIL_LIBRARY_BROWSER") != "1" {
		t.Skip("KINOSAIL_LIBRARY_BROWSER is not set")
	}
	media := t.TempDir()
	for number := 1; number <= 30; number++ {
		name := fmt.Sprintf("Pagination Show %02d", number)
		season := filepath.Join(media, name, "Season 01")
		if err := os.MkdirAll(season, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(season, name+" S01E01 Pilot.mp4"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for number := 1; number <= 6; number++ {
		if err := os.WriteFile(filepath.Join(media, fmt.Sprintf("Pagination Movie %02d.mp4", number)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	web := httptest.NewServer(server.New(server.Config{Lifecycle: t.Context(), MediaDir: media, DataDir: t.TempDir()}))
	t.Cleanup(web.Close)
	project := os.Getenv("KINOSAIL_BROWSER_PROJECT")
	if project == "" {
		project = "chromium"
	}
	command := exec.CommandContext(t.Context(), "pnpm", "exec", "playwright", "test", "library-pagination.spec.ts", "library-pagination-resilience.spec.ts", "--workers=1", "--project="+project) //nolint:gosec // Fixed test command and disposable local Server.
	command.Dir = "../../e2e"
	command.Env = append(os.Environ(), "KINOSAIL_LIBRARY_BROWSER_URL="+web.URL, "KINOSAIL_E2E_URL="+web.URL)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
}
