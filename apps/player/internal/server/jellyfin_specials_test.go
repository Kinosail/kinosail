package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

var jellyfinGUID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestJellyfinSpecialsSeasonHasItsOwnAddress(t *testing.T) {
	t.Parallel()
	media := t.TempDir()
	for _, name := range []string{
		"Severance/poster.jpg",
		"Severance/Season 00/Severance.S00E01.Main.Titles.mkv", "Severance/Season 00/Severance.S00E01.Main.Titles.jpg",
		"Severance/Season 01/Severance.S01E01.Good.News.mkv",
	} {
		path := filepath.Join(media, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := newJellyfinServer(t, server.Config{MediaDir: media, DataDir: t.TempDir(), RequireAuth: true})
	token, _ := jellyfinLogin(t, handler, signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password"))
	series := jellyfinPage(t, handler, token, "/Items?parentId=00000000000000000000000000000002").Items[0].ID
	seasons := jellyfinPage(t, handler, token, "/Shows/"+series+"/Seasons")
	if len(seasons.Items) != 2 || seasons.Items[0].Name != "Season 0" || seasons.Items[1].Name != "Season 1" {
		t.Fatalf("seasons = %#v", seasons.Items)
	}
	specials, regular := seasons.Items[0].ID, seasons.Items[1].ID
	if specials == series || specials == regular || !jellyfinGUID.MatchString(specials) {
		t.Fatalf("specials season ID %q, series %q, season 1 %q", specials, series, regular)
	}

	var season struct {
		ID          string `json:"Id"`
		Type        string `json:"Type"`
		IndexNumber int    `json:"IndexNumber"`
		ChildCount  int    `json:"ChildCount"`
	}
	decodeJellyfin(t, jellyfinCall(t, handler, http.MethodGet, "/Items/"+specials, "", token), &season)
	if season.ID != specials || season.Type != "Season" || season.IndexNumber != 0 || season.ChildCount != 1 {
		t.Fatalf("specials season = %#v", season)
	}
	for _, path := range []string{"/Shows/" + series + "/Episodes?seasonId=" + specials, "/Items?parentId=" + specials} {
		if names := jellyfinPageNames(jellyfinPage(t, handler, token, path)); len(names) != 1 || names[0] != "Episode:S00E01 · Main Titles" {
			t.Fatalf("%s = %q", path, names)
		}
	}
	var episodes struct {
		Items []struct {
			Name     string `json:"Name"`
			ParentID string `json:"ParentId"`
			SeasonID string `json:"SeasonId"`
		}
	}
	decodeJellyfin(t, jellyfinCall(t, handler, http.MethodGet, "/Shows/"+series+"/Episodes", "", token), &episodes)
	for _, episode := range episodes.Items {
		want := map[string]string{"S00E01 · Main Titles": specials, "S01E01 · Good News": regular}[episode.Name]
		if episode.SeasonID != want || episode.ParentID != want {
			t.Fatalf("episode %q season %q parent %q, want %q", episode.Name, episode.SeasonID, episode.ParentID, want)
		}
	}
	if image := jellyfinCall(t, handler, http.MethodGet, "/Items/"+specials+"/Images/Primary", "", token); image.Code != http.StatusOK || image.Body.String() != "Severance/Season 00/Severance.S00E01.Main.Titles.jpg" {
		t.Fatalf("specials image = %d %q", image.Code, image.Body.String())
	}

	var legacy struct{ Type string }
	decodeJellyfin(t, jellyfinCall(t, handler, http.MethodGet, "/Items/"+series, "", token), &legacy)
	if legacy.Type != "Series" {
		t.Fatalf("series ID resolves to %q", legacy.Type)
	}
	if names := jellyfinPageNames(jellyfinPage(t, handler, token, "/Shows/"+series+"/Episodes?seasonId="+series)); len(names) != 1 || names[0] != "Episode:S00E01 · Main Titles" {
		t.Fatalf("cached specials season ID episodes = %q", names)
	}
	if image := jellyfinCall(t, handler, http.MethodGet, "/Items/"+series+"/Images/Primary", "", token); image.Code != http.StatusOK || image.Body.String() != "Severance/poster.jpg" {
		t.Fatalf("series image = %d %q", image.Code, image.Body.String())
	}
}
