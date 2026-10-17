package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
)

func TestFailedLibraryScanKeepsMediaPathsOutOfViewerResponsesAndLogs(t *testing.T) {
	previous := slog.Default()
	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	root := filepath.Join(t.TempDir(), "Private Household Media")
	index := &libraryIndex{Index: catalog.NewDecoratedIndex(t.Context(), []catalog.ScanRoot{{Path: root, Namespace: "Movies"}}, "", time.Hour, nil, catalog.Decoration{})}
	if _, _, err := index.Status(); err == nil || !strings.Contains(err.Error(), root) {
		t.Fatalf("Owner scan status lost the cause: %v", err)
	}
	progress, lists := newProgressStore(""), newListStore("")
	home := httptest.NewRecorder()
	showHome(index, progress, lists, newSettingsStore("", "", "", nil), nil, false)(home, ownerRequest("/"))
	api := httptest.NewRecorder()
	catalogapi.Library(func(request *http.Request) (catalog.Result, error) {
		return browseLibrary(request, index, progress, lists)
	}, progress.ClientItem)(api, ownerRequest("/api/v1/library"))
	jellyfin := httptest.NewRecorder()
	(&jellyfinAPI{index: index}).items(jellyfin, ownerRequest("/Items"))
	for name, response := range map[string]*httptest.ResponseRecorder{"home": home, "api": api, "jellyfin": jellyfin} {
		body := response.Body.String()
		if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "" || strings.Contains(body, "Private Household Media") || strings.Contains(body, "no such file") || strings.Contains(body, "lstat") {
			t.Errorf("%s failure = %d Retry-After=%q %q", name, response.Code, response.Header().Get("Retry-After"), body)
		}
	}
	var failure struct{ Error string }
	if json.Unmarshal(api.Body.Bytes(), &failure) != nil || failure.Error != "library is unavailable" {
		t.Errorf("API failure message = %q", api.Body.String())
	}
	if !strings.Contains(home.Body.String(), "library is unavailable") {
		t.Errorf("home failure lacks the generic message: %q", home.Body.String())
	}
	var logged map[string]any
	for line := range strings.Lines(logs.String()) {
		var record map[string]any
		if json.Unmarshal([]byte(line), &record) == nil && record["operation"] == "library.scan" && record["outcome"] == "unavailable" {
			logged = record
		}
	}
	if logged == nil || logged["level"] != "ERROR" || logged["failure_class"] != "missing" {
		t.Errorf("scan failure log = %q", logs.String())
	}
	if strings.Contains(logs.String(), "Private Household Media") {
		t.Errorf("logs exposed the media path: %q", logs.String())
	}
}
