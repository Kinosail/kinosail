package servertest

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

type episodeTitleCase struct {
	path, title, show string
	season, episode   int
}

func (fixture LibraryAPIFixture) EpisodeFilenamesKeepEpisodeTitlesInAPI(t *testing.T) {
	mediaDir := t.TempDir()
	season := filepath.Join("Example Show", "Season 02")
	cases := []episodeTitleCase{
		{filepath.Join(season, "Example Show S02E01 Second Season Opener.mp4"), "S02E01 · Second Season Opener", "Example Show", 2, 1},
		{filepath.Join(season, "Example Show - S02E02 - The Long Night.mkv"), "S02E02 · The Long Night", "Example Show", 2, 2},
		{filepath.Join(season, "Example.Show.S02E03.The.Web.of.Lies.1080p.WEB.h264-GROUP.mkv"), "S02E03 · The Web of Lies", "Example Show", 2, 3},
		{filepath.Join(season, "Example Show 2x04 Fourth Light.mkv"), "S02E04 · Fourth Light", "Example Show", 2, 4},
		{filepath.Join(season, "Example.Show.S02E05.720p.HDTV.x264-GRP.mkv"), "S02E05", "Example Show", 2, 5},
		{filepath.Join(season, "Example Show S02E06 [1080p].mkv"), "S02E06", "Example Show", 2, 6},
		{filepath.Join(season, "Example Show S02E07 Raw Name.mp4"), "S02E07 · Local Name", "Example Show", 2, 7},
		{"Other.Show.S01E09.Pilot.2160p.WEB-DL.DDP5.1.HEVC-GRP.mkv", "S01E09 · Pilot", "Other Show", 1, 9},
		{"Third Show 3x12.mkv", "S03E12", "Third Show", 3, 12},
	}
	files := map[string]string{filepath.Join(season, "Example Show S02E07 Raw Name.nfo"): `<episodedetails><title>Local Name</title></episodedetails>`, "Example Movie (2020) [1920x1080].mkv": "video"}
	want := map[string]bool{"Example Movie||0|0": true}
	for _, item := range cases {
		files[item.path] = "video"
		want[fmt.Sprintf("%s|%s|%d|%d", item.title, item.show, item.season, item.episode)] = true
	}
	writeEpisodeFixtures(t, mediaDir, files)
	handler := fixture.NewHandler(mediaDir, "", false)
	response := APICall(t, handler, "", http.MethodGet, "/api/v1/library?limit=100", nil)
	var catalog struct {
		Items []struct {
			Title, Show     string
			Season, Episode int
		} `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil || response.Code != http.StatusOK {
		t.Fatalf("library = %d %q: %v", response.Code, response.Body.String(), err)
	}
	seen := make(map[string]bool, len(catalog.Items))
	for _, item := range catalog.Items {
		seen[fmt.Sprintf("%s|%s|%d|%d", item.Title, item.Show, item.Season, item.Episode)] = true
	}
	if !maps.Equal(seen, want) {
		t.Fatalf("library items = %v, want %v", slices.Sorted(maps.Keys(seen)), slices.Sorted(maps.Keys(want)))
	}
}

func writeEpisodeFixtures(t *testing.T, mediaDir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(mediaDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
