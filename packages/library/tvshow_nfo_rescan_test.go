package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRescanAppliesAChangedTVShowNFOToItsEpisodes(t *testing.T) {
	root := t.TempDir()
	show := filepath.Join(root, "Series")
	season := filepath.Join(show, "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(season, "Series - S01E01 - Pilot.mkv"), "video")
	scanner := NewScanner(root, "Shows")
	items, err := scanner.Scan(t.Context())
	if err != nil || len(items) != 1 || items[0].ShowGenres != "" || items[0].Rating != "" || items[0].ShowTitle != "Series" {
		t.Fatalf("initial scan = %+v, %v", items, err)
	}
	write(filepath.Join(show, "tvshow.nfo"), `<tvshow><title>Renamed</title><mpaa>TV-PG</mpaa><genre>Drama</genre></tvshow>`)
	items, err = scanner.Rescan(t.Context(), []string{show})
	if err != nil || len(items) != 1 || items[0].ShowGenres != "Drama" || items[0].Rating != "TV-PG" || items[0].ShowTitle != "Renamed" || items[0].Show != "Series" {
		t.Fatalf("rescan after adding tvshow.nfo = %+v, %v", items, err)
	}
	if err := os.Remove(filepath.Join(show, "tvshow.nfo")); err != nil {
		t.Fatal(err)
	}
	items, err = scanner.Rescan(t.Context(), []string{show})
	if err != nil || len(items) != 1 || items[0].ShowGenres != "" || items[0].Rating != "" || items[0].ShowTitle != "Series" {
		t.Fatalf("rescan after removing tvshow.nfo = %+v, %v", items, err)
	}
}
