package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

var snapshotProfiles = []viewerProfile{
	{ID: "owner", Owner: true},
	{ID: "teen", Rating: "teen", Libraries: []string{"all"}},
	{ID: "family", Rating: "family", Libraries: []string{"all"}},
	{ID: "default", Libraries: []string{"Library"}},
	{ID: "everything", Rating: "all", Libraries: []string{"Library", "all"}},
	{ID: "elsewhere", Rating: "all", Libraries: []string{"Other"}},
}

func snapshotRootCount(items []library.Item, profile viewerProfile) int {
	count, shows := 0, make(map[string]bool)
	for _, item := range items {
		if !viewerPolicy(profile).Allows(item) {
			continue
		}
		if item.Kind == "video" && item.Show != "" {
			shows[strings.ToLower(item.Show)] = true
			continue
		}
		count++
	}
	return count + len(shows)
}

func snapshotTargets(items []library.Item) []string {
	movie, episode, track := items[300], items[305], items[309]
	show, album := library.ShowID(episode.Show), library.NewSnapshot(items, 1).View(library.NewPolicy(true, "", nil)).Albums()[0].ID
	return []string{
		"/", "/?view=shows", "/?view=movies&sort=added", "/?view=collections", "/?view=playlists",
		"/watch/" + movie.ID, "/watch/" + episode.ID, "/watch/" + track.ID, "/show/" + show, "/album/" + album, "/actor?name=Nobody",
		"/api/v1/shows", "/api/v1/shows/" + show, "/api/v1/albums", "/api/v1/albums/" + album, "/api/v1/history",
		"/api/v1/audio/" + track.ID + "/queue", "/api/v1/library?view=history", "/api/v1/library?sort=year", "/api/v1/playlists",
		"/Items", "/Items?IncludeItemTypes=Episode&startIndex=3&limit=5", "/Items?searchTerm=movie", "/Items/Latest?limit=4",
		"/Items/" + movie.ID, "/Items/" + show + "0000000000000000", "/Shows/" + show + "/Seasons", "/Shows/" + show + "/Episodes",
		"/Shows/NextUp", "/UserItems/Resume", "/UserViews", "/Items/" + show + "0000000000000000/Images/Primary",
	}
}

func TestLibraryRequestsLeaveThePublishedSnapshotUnchanged(t *testing.T) {
	t.Parallel()
	items := snapshotFixtureItems(600)
	fixture := newSnapshotFixture(t, items, 300, 305, 309)
	published, _ := fixture.index.Snapshot()
	for position, profile := range snapshotProfiles {
		for _, item := range published[position*40 : position*40+20] {
			if err := fixture.progress.Set(snapshotRequest(t.Context(), http.MethodPost, "/", profile), item.ID, float64(position+1), nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	var requests sync.WaitGroup
	for range 2 {
		for _, profile := range snapshotProfiles {
			for _, target := range snapshotTargets(published) {
				requests.Go(func() {
					response := httptest.NewRecorder()
					fixture.mux.ServeHTTP(response, snapshotRequest(t.Context(), http.MethodGet, target, profile))
					if response.Code != http.StatusOK && response.Code != http.StatusNotFound {
						t.Errorf("%s %s = %d", profile.ID, target, response.Code)
					}
				})
			}
		}
	}
	requests.Wait()
	if current, _ := fixture.index.Snapshot(); !reflect.DeepEqual(current, published) {
		t.Fatal("requests changed the published Library snapshot")
	}
	for range 2 {
		for _, profile := range snapshotProfiles {
			response := httptest.NewRecorder()
			fixture.mux.ServeHTTP(response, snapshotRequest(t.Context(), http.MethodGet, "/Items?limit=1", profile))
			var page struct{ TotalRecordCount int }
			if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || page.TotalRecordCount != snapshotRootCount(published, profile) {
				t.Fatalf("%s root count = %d, want %d (%v)", profile.ID, page.TotalRecordCount, snapshotRootCount(published, profile), err)
			}
		}
	}
}
