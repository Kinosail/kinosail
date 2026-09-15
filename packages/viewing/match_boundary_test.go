package viewing

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestMovieIdentityRequiresBothYears(t *testing.T) {
	if sameIdentity(Activity{Kind: "movie", Title: "Movie", Year: "2024"}, library.Item{Kind: "video", Title: "Movie"}) {
		t.Fatal("movie identity accepted without a destination year")
	}
}
