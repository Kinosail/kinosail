package catalogapi

import (
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestActorLibraryExactCreditsAndGroupedShows(t *testing.T) {
	cast := []library.Person{{Name: "Zoë Actor", Role: "Lead", Image: "/private/face.jpg"}}
	items := []library.Item{
		{ID: "movie", Kind: "video", Title: "Film", Cast: cast, Artwork: "/private/poster.jpg"},
		{ID: "e1", Kind: "video", Show: "Series", Season: 1, Episode: 1, ShowCast: cast, ShowArtwork: "/private/show.jpg"},
		{ID: "e2", Kind: "video", Show: "Series", Season: 1, Episode: 2, Cast: []library.Person{{Name: "Zoë Actor", Role: "Guest"}}, ShowCast: cast},
		{ID: "unrelated", Kind: "video", Title: "Zoë Actor", Cast: []library.Person{{Name: "Other", Role: "Zoë Actor"}}},
		{ID: "partial", Kind: "video", Cast: []library.Person{{Name: "Zoë Actor Jr."}}},
		{ID: "photo", Kind: "photo", Cast: cast},
	}
	page, err := ActorLibrary(items, "  zoe\u0308   actor ")
	if err != nil || len(page.Movies) != 1 || len(page.Shows) != 1 {
		t.Fatalf("page = %#v, %v", page, err)
	}
	if page.Movies[0].URL != "/watch/movie" || page.Shows[0].Role != "Lead · Guest" || page.Image != "/person/movie/0" || page.Shows[0].Artwork != "/art/e1" {
		t.Fatalf("credits = %#v", page)
	}
	if strings.Contains(page.Image, "private") {
		t.Fatal("private image path leaked")
	}
}

func TestActorLibraryUsesOnlySuppliedVisibleItems(t *testing.T) {
	page, err := ActorLibrary(nil, "Hidden Actor")
	if err != nil || len(page.Movies) != 0 || len(page.Shows) != 0 || page.Image != "" {
		t.Fatalf("hidden actor leaked: %#v %v", page, err)
	}
	page, err = ActorLibrary([]library.Item{{ID: "e", Kind: "video", Show: "Series", ShowCast: []library.Person{{Name: "Actor"}}}}, "Actor")
	if err != nil || page.Image != "" || len(page.Shows) != 1 || page.Shows[0].Artwork != "" {
		t.Fatalf("missing artwork = %#v %v", page, err)
	}
}

func TestActorQueryRejectsMalformedAndAmbiguousInput(t *testing.T) {
	for _, raw := range []string{"", "name=", "name=%20", "name=Actor&name=Other", "name=Actor&extra=1", "name=%zz", "name=%ff", "name=Actor%00", "name=" + strings.Repeat("a", 201), strings.Repeat("a", 2049)} {
		if _, err := ActorQuery(raw); err == nil {
			t.Errorf("accepted query %q", raw)
		}
	}
	name := "A & B + O’Neil"
	if got, err := ActorQuery(url.Values{"name": {name}}.Encode()); err != nil || got != name {
		t.Fatalf("special name = %q %v", got, err)
	}
	for _, name := range []string{"", strings.Repeat("a", 201), "bad\nname", string([]byte{255})} {
		if _, err := ActorLibrary(nil, name); err == nil {
			t.Errorf("accepted name %q", name)
		}
	}
}

func TestActorLibrarySortsMoviesAndScopesShowPortraits(t *testing.T) {
	cast := []library.Person{{Name: "Actor", Role: "Lead", Image: "/private/show-face.jpg"}}
	page, err := ActorLibrary([]library.Item{
		{ID: "episode", Kind: "video", Show: "Series", Season: 1, Episode: 1, ShowCast: cast},
		{ID: "z", Kind: "video", Title: "Film", Cast: cast},
		{ID: "a", Kind: "video", Title: "Film", Cast: cast},
		{ID: "first", Kind: "video", Title: "A Film", Cast: cast},
	}, "Actor")
	if err != nil || page.Image != "/person/episode/0?scope=show" || len(page.Movies) != 3 {
		t.Fatalf("actor projection = %#v, %v", page, err)
	}
	for i, id := range []string{"first", "a", "z"} {
		if page.Movies[i].ID != id || page.Movies[i].Artwork != "" {
			t.Fatalf("movie %d = %#v", i, page.Movies[i])
		}
	}
}
