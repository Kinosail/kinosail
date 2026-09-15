package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalogapi"
)

func TestActorRejectsQueryBeforeLibraryAccess(t *testing.T) {
	for _, api := range []bool{true, false} {
		for _, query := range []string{"", "name=", "name=A&name=B", "name=%00", "name=A&extra=1"} {
			response := httptest.NewRecorder()
			browseActor(nil, api)(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/actor?"+query, nil))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid query = %d", response.Code)
			}
		}
	}
}

func TestActorTemplateEscapesCreditsAndLinksTitles(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/actor?name=Actor", nil)
	response := httptest.NewRecorder()
	page := catalogapi.ActorPage{Name: "Actor <script>", Movies: []catalogapi.ActorTitle{{Title: "Film & friends", URL: "/watch/movie", Role: "Lead"}}, Shows: []catalogapi.ActorTitle{{Title: "Series", URL: "/show/series", Artwork: "/art/episode"}}}
	if err := actorView.Execute(response, request, page); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Actor &lt;script&gt;", "Film &amp; friends", `href="/watch/movie"`, `href="/show/series"`, "Lead"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("missing %q", expected)
		}
	}
}
