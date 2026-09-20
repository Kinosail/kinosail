package downloads

import (
	"net/http"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestPreparedDownloadsRecheckCurrentItemVisibility(t *testing.T) {
	job := Job{ID: "aaaaaaaaaaaaaaaa", Profile: "viewer", ItemID: "item", State: "ready", Title: "Private title"}
	manager := &Manager{jobs: map[string]Job{job.ID: job}}
	visible := true
	access := NewAccess(func(*http.Request) identitycore.Profile { return identitycore.Profile{ID: "viewer", Downloads: true} }, func(*http.Request, string) (library.Item, bool) { return library.Item{ID: "item"}, visible })
	mux := http.NewServeMux()
	RegisterAPI(mux, manager, access)
	if response := requestAPI(t, mux, http.MethodGet, "/api/v1/downloads/"+job.ID, "", true); response.Code != http.StatusOK {
		t.Fatal(response.Code)
	}
	visible = false
	for _, suffix := range []string{"", "/file", "/manifest"} {
		response := requestAPI(t, mux, http.MethodGet, "/api/v1/downloads/"+job.ID+suffix, "", true)
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), job.Title) {
			t.Fatalf("revoked item exposed: %d %s", response.Code, response.Body.String())
		}
	}
	response := requestAPI(t, mux, http.MethodGet, "/api/v1/downloads", "", true)
	if strings.Contains(response.Body.String(), job.Title) || len(manager.jobs) != 1 {
		t.Fatal("listing exposed or mutated a revoked item")
	}
}
