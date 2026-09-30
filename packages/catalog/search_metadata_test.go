package catalog

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestMetadataSearchPreservesFieldBoundariesAndNormalization(t *testing.T) {
	item := library.Item{
		Title: "THE...Signal", Show: "Northern Lights", Year: "2026", Plot: "A\tquiet\njourney",
		Genres: "Drama/Mystery", Director: "Alex North", Studio: "Harbor", Artist: "Isla", Album: "After Rain",
		Cast: []library.Person{{Name: "Sam Reed", Role: "Captain"}}, ShowCast: []library.Person{{Name: "Morgan Vale", Role: "Guide"}},
	}
	for _, query := range []string{
		"the signal", "signal northern", "lights 2026 a quiet", "journey drama mystery",
		"mystery alex", "north harbor isla", "after rain sam reed", "captain morgan vale guide", "SIGNAL!!!", "ＳＩＧＮＡＬ",
	} {
		if !Matches(item, query) {
			t.Errorf("metadata query %q did not match", query)
		}
	}
	for _, query := range []string{"", "!!!", "signal2026", "signal harbor", "rain captain", "signal cafe", "café"} {
		if Matches(item, query) {
			t.Errorf("unmatched query %q matched", query)
		}
	}
}

func TestMetadataSearchNormalizesEveryUnicodeField(t *testing.T) {
	for field, unicodeItem := range map[string]library.Item{
		"title":     {Title: "Café"},
		"show":      {Show: "Café"},
		"year":      {Year: "２０２６ Café"},
		"plot":      {Plot: "Café"},
		"genres":    {Genres: "Café"},
		"director":  {Director: "Café"},
		"studio":    {Studio: "Café"},
		"artist":    {Artist: "Café"},
		"album":     {Album: "Café"},
		"cast":      {Cast: []library.Person{{Name: "Other", Role: "Café"}}},
		"show-cast": {ShowCast: []library.Person{{Name: "Café", Role: "Other"}}},
	} {
		t.Run(field, func(t *testing.T) {
			if !Matches(unicodeItem, "CAFE") || Matches(unicodeItem, "cafe missing") {
				t.Fatal("Unicode metadata changed matching behavior")
			}
		})
	}
}

func TestMetadataSearchPreservesLargeAndCompatibilityValues(t *testing.T) {
	long := library.Item{Title: strings.Repeat("Long-Title ", 100), Plot: "Ends_here"}
	if !Matches(long, "title ends here") || Matches(long, "title ends nowhere") {
		t.Fatal("large metadata lost its tail or matched an absent value")
	}
	for _, test := range []struct{ title, query string }{
		{"Az\x00b\x7fc", "az b c"},
		{"A\xffB", "a b"},
		{strings.Repeat("A journey ", 120) + "CAFÉ", "journey cafe"},
		{strings.Repeat("㌖", 200), "キロメートルキロメートル"},
		{"℉", "℉"},
		{"𝐀", "𝐀"},
	} {
		if !Matches(library.Item{Title: test.title}, test.query) || Matches(library.Item{Title: test.title}, "absent token") {
			t.Errorf("normalization failed for query %q", test.query)
		}
	}
	if Matches(library.Item{Title: "℉"}, "F") || Matches(library.Item{Title: "𝐀"}, "A") {
		t.Fatal("compatibility characters changed the lowercase-before-decomposition contract")
	}
}
