package metadata

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestTMDBIdentityAndValidation(t *testing.T) {
	t.Parallel()
	if title, year := MovieIdentity(library.Item{Title: " Movie [2020] ", Year: ""}); title != "Movie" || year != "2020" {
		t.Fatalf("identity = %q, %q", title, year)
	}
	if title, year := MovieIdentity(library.Item{Title: "Movie (2020)", Year: "1999"}); title != "Movie" || year != "1999" {
		t.Fatalf("identity with local year = %q, %q", title, year)
	}
	if title, year := MovieIdentity(library.Item{Title: "1917"}); title != "1917" || year != "" {
		t.Fatalf("numeric title identity = %q, %q", title, year)
	}
	for _, id := range []string{"tt1234567", "tt123456789"} {
		if !ValidIMDbID(id) {
			t.Fatalf("valid IMDb ID %q rejected", id)
		}
	}
	for _, id := range []string{"", "tt123", "nm1234567", "tt1234567890"} {
		if ValidIMDbID(id) {
			t.Fatalf("invalid IMDb ID %q accepted", id)
		}
	}
	if !ValidTMDBPath("") || !ValidTMDBPath("/poster.jpg") || ValidTMDBPath("poster.jpg") || ValidTMDBPath("//host/path") || ValidTMDBPath("/../poster.jpg") || ValidTMDBPath("/%2e%2e/poster.jpg") || ValidTMDBPath("/"+strings.Repeat("a", 2048)) {
		t.Fatal("TMDB path validation mismatch")
	}
	if !ValidTMDBCast([]TMDBCastMember{{Name: "Actor", ProfilePath: "/actor.jpg"}}) || ValidTMDBCast([]TMDBCastMember{{Name: strings.Repeat("x", 201)}}) {
		t.Fatal("TMDB cast validation mismatch")
	}
}
