package server_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

const jellyfinShapeGolden = "testdata/jellyfin-response-shape.json"

var updateJellyfinShape = flag.Bool("update-jellyfin-shape", false, "rewrite "+jellyfinShapeGolden)

type jellyfinShapeSession struct {
	name, token string
	paths       []string
}

func TestJellyfinListingsKeepClientResponseShape(t *testing.T) {
	t.Parallel()
	handler, sessions, ids := jellyfinShapeServer(t)
	placeholders := make([]string, 0, 2*len(ids))
	for name, id := range ids {
		placeholders = append(placeholders, "{"+name+"}", id)
	}
	expand := strings.NewReplacer(placeholders...)
	responses := make(map[string]any)
	for _, session := range sessions {
		for _, path := range session.paths {
			response := jellyfinCall(t, handler, http.MethodGet, expand.Replace(path), "", session.token)
			decoder := json.NewDecoder(response.Body)
			decoder.UseNumber()
			var body any
			if err := decoder.Decode(&body); err != nil {
				t.Fatalf("%s %s = %d %q: %v", session.name, path, response.Code, response.Body.String(), err)
			}
			responses[session.name+" GET "+path] = map[string]any{"status": response.Code, "contentType": response.Header().Get("Content-Type"), "body": body}
		}
	}
	got, err := json.MarshalIndent(responses, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if *updateJellyfinShape {
		if err := os.WriteFile(jellyfinShapeGolden, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(jellyfinShapeGolden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		gotLines, wantLines := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
		for line := 0; line < min(len(gotLines), len(wantLines)); line++ {
			if gotLines[line] != wantLines[line] {
				t.Fatalf("Jellyfin response shape changed at %s:%d\n got: %s\nwant: %s", jellyfinShapeGolden, line+1, gotLines[line], wantLines[line])
			}
		}
		t.Fatalf("Jellyfin response shape changed: %d lines, want %d", len(gotLines), len(wantLines))
	}
}

func jellyfinShapeServer(t *testing.T) (http.Handler, []jellyfinShapeSession, map[string]string) {
	t.Helper()
	media := t.TempDir()
	writeJellyfinShapeMedia(t, media)
	handler := newJellyfinServer(t, server.Config{MediaDir: media, DataDir: t.TempDir(), RequireAuth: true})
	ownerCookie := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	if created := requestWithCookie(t, handler, http.MethodPost, "/settings/profiles", "name=Teen&password=teen-password&rating=teen&libraries=all", ownerCookie); created.Code >= http.StatusBadRequest {
		t.Fatalf("create Viewer Profile = %d %q", created.Code, created.Body.String())
	}
	owner, ownerID := jellyfinLogin(t, handler, ownerCookie)
	viewer, _ := jellyfinLogin(t, handler, signInTestProfile(t, handler, "/login", "name=Teen&password=teen-password"))
	ids := map[string]string{"user": ownerID, "movies": "00000000000000000000000000000001", "shows": "00000000000000000000000000000002"}
	for _, path := range []string{"/Items", "/Items?includeItemTypes=Episode"} {
		for _, item := range jellyfinPage(t, handler, owner, path).Items {
			ids[item.Name] = item.ID
		}
	}
	for _, item := range jellyfinPage(t, handler, owner, "/Items?parentId="+ids["Severance"]).Items {
		ids["Severance "+item.Name] = item.ID
	}
	for _, call := range []struct{ method, path, body string }{
		{http.MethodPost, "/Sessions/Playing/Progress", `{"ItemId":"` + ids["Arrival"] + `","PositionTicks":100000000}`},
		{http.MethodPost, "/UserPlayedItems/" + ids["S01E01 · Good News"], ""},
		{http.MethodPost, "/UserFavoriteItems/" + ids["S01E02 · Half Loop"], ""},
	} {
		if response := jellyfinCall(t, handler, call.method, call.path, call.body, owner); response.Code >= http.StatusBadRequest {
			t.Fatalf("%s %s = %d %q", call.method, call.path, response.Code, response.Body.String())
		}
	}
	return handler, []jellyfinShapeSession{
		{name: "owner", token: owner, paths: []string{
			"/UserViews", "/Items", "/Items?includeItemTypes=Episode", "/Items?parentId={movies}", "/Items?parentId={shows}",
			"/Items?parentId={Severance}", "/Items?parentId={Severance Season 1}", "/Items?parentId=missing",
			"/Items?includeItemTypes=Episode&searchTerm=s0&limit=2&startIndex=1", "/Items?ids={Arrival},{S01E01 · Good News}", "/Items/Latest?limit=4",
			"/Items/Latest?parentId=missing", "/Items/{Arrival}", "/Users/{user}/Items/{S01E01 · Good News}", "/Items/{Severance}",
			"/Items/{Severance Season 1}", "/Shows/{Severance}/Seasons", "/Shows/{Severance}/Episodes", "/Shows/{Severance}/Episodes?season=2",
			"/Shows/NextUp", "/UserItems/Resume", "/Persons",
		}},
		{name: "teen", token: viewer, paths: []string{"/Items", "/Items?includeItemTypes=Episode", "/Items/Latest", "/Items/{Arrival}"}},
	}, ids
}

func writeJellyfinShapeMedia(t *testing.T, media string) {
	t.Helper()
	files := map[string]string{
		"Arrival.nfo":          `<movie><title>Arrival</title><year>2016</year><plot>A linguist meets visitors.</plot><mpaa>PG-13</mpaa><genre>Drama</genre><genre>Science Fiction</genre><uniqueid type="imdb">tt2543164</uniqueid><uniqueid type="tmdb">329865</uniqueid></movie>`,
		"Arrival.jpg":          "poster",
		"Arrival.srt":          "1\n00:00:01,000 --> 00:00:02,000\nHello\n",
		"Arrival.fr.srt":       "1\n00:00:01,000 --> 00:00:02,000\nBonjour\n",
		"Heat (1995).mkv":      "video",
		"Heat (1995).nfo":      `<movie><mpaa>R</mpaa></movie>`,
		"Untitled Short.webm":  "video",
		"Severance/poster.jpg": "poster",
		"Severance/Season 01/Severance.S01E01.Good.News.jpg": "thumb",
		"Severance/Season 01/Severance.S01E01.Good.News.mkv": "video",
		"Severance/Season 01/Severance.S01E02.Half.Loop.mkv": "video",
		"Boards of Canada/Signal/01 Reach for the Dead.mp3":  "audio",
		"Boards of Canada/Signal/01 Reach for the Dead.nfo":  `<song><title>Reach for the Dead</title><artist>Boards of Canada</artist><album>Signal</album><track>1</track></song>`,
		"Arrival.mp4": "video",
		"Severance/Season 01/Severance.S01E02.Half.Loop.nfo":                   `<episodedetails><title>Half Loop</title><mpaa>TV-14</mpaa></episodedetails>`,
		"Severance/Season 00/Severance.S00E01.Main.Titles.mkv":                 "video",
		"Severance/Season 02/Severance.S02E01.Hello.Ms.Cobel.mkv":              "video",
		"Pluribus (2025) {tvdb-123456}/Season 01/Pluribus.S01E01.We.Is.Us.mkv": "video",
		"Photos/Coast.jpg": "photo",
	}
	modified := time.Date(2026, time.March, 1, 9, 0, 0, 0, time.UTC)
	for _, name := range slices.Sorted(maps.Keys(files)) {
		path := filepath.Join(media, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(files[name]), 0o600); err != nil {
			t.Fatal(err)
		}
		modified = modified.Add(time.Hour)
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
}
