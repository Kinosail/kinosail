package catalogapi

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestShowCastUsesVisibleEpisodeAndPublicImagePath(t *testing.T) {
	show := library.Show{Episodes: []library.Item{{ID: "empty"}, {ID: "episode", ShowCast: []library.Person{{Name: "Actor", Role: "Lead", Image: "/private/cache/face.jpg"}, {Name: "Voice"}}}}}
	cast := ShowCast(show)
	if len(cast) != 2 || cast[0].Image != "/person/episode/0?scope=show" || cast[0].Role != "Lead" || cast[1].Image != "" {
		t.Fatalf("cast = %#v", cast)
	}
	if len(ShowCast(library.Show{Cast: show.Episodes[1].ShowCast})) != 0 {
		t.Fatal("cast exposed without visible episode")
	}
}

func TestShowAPIIncludesCastWithoutCachePaths(t *testing.T) {
	items := []library.Item{{ID: "episode", Kind: "video", Show: "Series", ShowTitle: "Series", Season: 1, Episode: 1, ShowCast: []library.Person{{Name: "Actor", Image: "/private/cache/face.jpg"}}}}
	handlers, _, _, _ := mediaHandlersForTest(items)
	_, shows := library.Organize(items)
	response := callMedia(t, registeredMediaHandler(handlers), "GET", "/api/v1/shows/"+shows[0].ID, "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"name":"Actor"`) || strings.Contains(response.Body.String(), "/private/cache") {
		t.Fatalf("show API = %d %s", response.Code, response.Body.String())
	}
}
