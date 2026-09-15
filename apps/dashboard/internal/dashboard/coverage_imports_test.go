package dashboard

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCoverageImportFailuresDoNotMutate(t *testing.T) {
	for _, mode := range []string{"title", "app", "version", "storage"} {
		t.Run(mode, func(t *testing.T) {
			service, store := serviceForTest(t, testBoard())
			input := Import{Title: "Home", ExpectedVersion: 1}
			switch mode {
			case "title":
				input.Title = ""
			case "app":
				input.Apps = []App{{}}
			case "version":
				input.ExpectedVersion = 2
			case "storage":
				store.failSave = true
			}
			before := service.BoardExport()
			if _, err := service.ImportBoard(t.Context(), input, "Owner"); err == nil {
				t.Fatal("invalid import accepted")
			}
			assertCoverageBoardUnchanged(t, service, before)
			if mode != "storage" && store.saveCount != 0 {
				t.Fatal("rejected import saved")
			}
		})
	}
}

func TestCoverageExternalImportFailuresDoNotMutate(t *testing.T) {
	for _, mode := range []string{"content", "entropy", "version", "storage", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			service, store := serviceForTest(t, testBoard())
			input := ExternalImport{Source: "homarr", Content: `{"apps":[{"name":"Media","url":"https://media.test"}]}`}
			version := uint64(1)
			switch mode {
			case "content":
				input.Content = ""
			case "entropy":
				service.randomID = func() (string, error) { return "", errors.New("entropy unavailable") }
			case "version":
				version = 2
			case "storage":
				store.failSave = true
			case "duplicate":
				service.board.Apps = []App{testApp(1, "https://media.test", false)}
			}
			before := service.BoardExport()
			if _, err := service.ImportExternal(t.Context(), input, version, "Owner"); err == nil {
				t.Fatal("external import accepted failure")
			}
			assertCoverageBoardUnchanged(t, service, before)
			if mode != "storage" && store.saveCount != 0 {
				t.Fatal("rejected external import saved")
			}
		})
	}
}

func TestCoverageImportNormalizationAndCapacity(t *testing.T) {
	service, _ := serviceForTest(t, testBoard())
	candidate := ImportedApp{Name: "Media", URL: "https://media.test", Icon: "app", Accent: "slate"}
	apps, err := service.normalizeImportedApps([]ImportedApp{{}, candidate, candidate}, time.Now())
	if err != nil || len(apps) != 1 || apps[0].URL != candidate.URL {
		t.Fatalf("normalized import = %+v, %v", apps, err)
	}
	if added := mergeImportedApps(make([]App, MaxApps), apps, map[string]bool{}); len(added) != 0 {
		t.Fatal("full board accepted imported apps")
	}
	var items []string
	for index := 0; index <= MaxApps; index++ {
		items = append(items, fmt.Sprintf(`{"name":"App%d","url":"https://app%d.test"}`, index, index))
	}
	if _, err := PreviewExternal(ExternalImport{Source: "homarr", Content: `{"apps":[` + strings.Join(items, ",") + `]}`}); err == nil {
		t.Fatal("oversized import accepted")
	}
}

func TestCoverageImportedNamesAndCatalogHints(t *testing.T) {
	app := importedApp("", "https://media.test", "", "")
	if app.Name != "media.test" {
		t.Fatalf("fallback name = %q", app.Name)
	}
	entry, _ := catalogByID("jellyfin")
	app = importedApp("Jellyfin", "https://media.test", "", "")
	if app.Description != entry.Description || app.Category != entry.Category || app.Icon != entry.Icon {
		t.Fatalf("catalog hints = %+v", app)
	}
	if got := firstString(map[string]any{}, "name", "title"); got != "" {
		t.Fatalf("missing string = %q", got)
	}
	if got := compactApps([]ImportedApp{{Name: "", URL: "https://media.test"}, {Name: "Media", URL: "https://media.test"}, {Name: "Again", URL: "https://media.test"}}); len(got) != 1 {
		t.Fatalf("compacted = %+v", got)
	}
}
