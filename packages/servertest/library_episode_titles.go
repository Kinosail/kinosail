package servertest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
	for _, item := range cases {
		path := filepath.Join(mediaDir, item.path)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	nfo := filepath.Join(mediaDir, season, "Example Show S02E07 Raw Name.nfo")
	if err := os.WriteFile(nfo, []byte(`<episodedetails><title>Local Name</title></episodedetails>`), 0o600); err != nil {
		t.Fatal(err)
	}
	movie := "Example Movie (2020) [1920x1080].mkv"
	if err := os.WriteFile(filepath.Join(mediaDir, movie), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(mediaDir, "", false)
	response := APICall(t, handler, "", http.MethodGet, "/api/v1/library?limit=100", nil)
	var catalog struct {
		Items []struct {
			Title, Show     string
			Season, Episode int
		} `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil || response.Code != http.StatusOK || len(catalog.Items) != len(cases)+1 {
		t.Fatalf("library = %d %q: %v", response.Code, response.Body.String(), err)
	}
	found := make(map[string]bool)
	seen := make([]string, 0, len(catalog.Items))
	for _, item := range catalog.Items {
		seen = append(seen, fmt.Sprintf("%s|%s|%d|%d", item.Title, item.Show, item.Season, item.Episode))
		if item.Title == "Example Movie" && item.Show == "" {
			found["movie"] = true
		}
		for _, want := range cases {
			if item.Title == want.title && item.Show == want.show && item.Season == want.season && item.Episode == want.episode {
				found[want.path] = true
			}
		}
	}
	for _, want := range cases {
		if !found[want.path] {
			t.Errorf("episode %q missing title %q show %q S%dE%d", want.path, want.title, want.show, want.season, want.episode)
		}
	}
	if !found["movie"] {
		t.Error("movie with a resolution was parsed as an episode")
	}
	if t.Failed() {
		t.Logf("library items: %q", seen)
	}
}
