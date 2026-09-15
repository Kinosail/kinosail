package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestJellyfinShowIdentityKeepsEpisodesMappedToTheirSeries(t *testing.T) {
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	wanted := map[string]string{
		"Fallout":                        "fallout episode",
		"A Knight of the Seven Kingdoms": "knight episode",
	}
	for show, contents := range wanted {
		suffix := map[string]string{"Fallout": " (2024) {imdb-tt12637874}", "A Knight of the Seven Kingdoms": " (2026) {imdb-tt27497448}"}[show]
		path := filepath.Join(mediaDir, show+suffix, "Season 01", "Episode.S01E01.mkv")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	handler := newJellyfinServer(t, server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	token, _ := jellyfinLogin(t, handler, owner)
	seriesResponse := jellyfinCall(t, handler, http.MethodGet, "/Items?ParentId=00000000000000000000000000000002&IncludeItemTypes=Series", "", token)
	var series struct {
		Items []struct {
			ID, Name string
		} `json:"Items"`
	}
	decodeJellyfin(t, seriesResponse, &series)
	if seriesResponse.Code != http.StatusOK || len(series.Items) != len(wanted) {
		t.Fatalf("series = %d %q", seriesResponse.Code, seriesResponse.Body.String())
	}

	for showName, expectedContents := range wanted {
		var showID string
		for _, item := range series.Items {
			if item.Name == showName {
				showID = item.ID
			}
		}
		if showID == "" {
			t.Fatalf("series %q missing from %q", showName, seriesResponse.Body.String())
		}
		assertJellyfinSeriesPlayback(t, handler, token, showID, showName, expectedContents)
	}
}

func assertJellyfinSeriesPlayback(t *testing.T, handler http.Handler, token, showID, showName, expectedContents string) {
	t.Helper()
	episodesResponse := jellyfinCall(t, handler, http.MethodGet, "/Shows/"+showID+"/Episodes?Season=1", "", token)
	var episodes struct {
		Items []struct {
			ID, SeriesID, SeriesName string
		} `json:"Items"`
	}
	decodeJellyfin(t, episodesResponse, &episodes)
	if episodesResponse.Code != http.StatusOK || len(episodes.Items) != 1 || episodes.Items[0].SeriesID != showID || episodes.Items[0].SeriesName != showName {
		t.Fatalf("%s episodes = %d %q", showName, episodesResponse.Code, episodesResponse.Body.String())
	}
	playback := jellyfinCall(t, handler, http.MethodPost, "/Items/"+episodes.Items[0].ID+"/PlaybackInfo", `{}`, token)
	var play struct {
		PlaySessionID string `json:"PlaySessionId"`
	}
	decodeJellyfin(t, playback, &play)
	stream := jellyfinCall(t, handler, http.MethodGet, "/Videos/"+episodes.Items[0].ID+"/stream?playSessionId="+play.PlaySessionID, "", token)
	if playback.Code != http.StatusOK || stream.Code != http.StatusOK || stream.Body.String() != expectedContents {
		t.Fatalf("%s playback = %d %q, stream = %d %q", showName, playback.Code, playback.Body.String(), stream.Code, stream.Body.String())
	}
}
