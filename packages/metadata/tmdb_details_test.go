package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

type tmdbEpisodeCase struct {
	name, body string
	episode    int
	ids        map[string]string
	failed     bool
	want       Record
	poster     string
}

func TestTMDBEpisodeDetails(t *testing.T) {
	t.Parallel()
	for _, test := range []tmdbEpisodeCase{
		{"series only", "", 0, nil, false, Record{Title: "Series", Year: "2020", Plot: "original"}, "original.jpg"},
		{"unavailable", "", 2, nil, true, Record{Title: "Series", Year: "2020", Plot: "original"}, "original.jpg"},
		{"episode", `{"id":9,"name":"Arrival","overview":"Episode plot","air_date":"2021-01-02","still_path":"/still.jpg"}`, 2, nil, false, Record{Title: "S01E02 · Arrival", Year: "2021", Plot: "Episode plot", ProviderIDs: map[string]string{"tmdb": "9"}}, "/still.jpg"},
		{"preserve provider and poster", `{"id":9,"name":"Arrival","overview":"Episode plot","air_date":"2021-01-02"}`, 2, map[string]string{"tvdb": "7"}, false, Record{Title: "S01E02 · Arrival", Year: "2021", Plot: "Episode plot", ProviderIDs: map[string]string{"tmdb": "9", "tvdb": "7"}}, "original.jpg"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertTMDBEpisodeDetails(t, test)
		})
	}
}

func assertTMDBEpisodeDetails(t *testing.T, test tmdbEpisodeCase) {
	t.Helper()
	calls := 0
	provider := TMDBDetails{BaseURL: "https://provider.example///", Fetch: func(ctx context.Context, endpoint string, target any) error {
		calls++
		if ctx != t.Context() || endpoint != "https://provider.example/tv/3/season/1/episode/2" {
			t.Fatalf("unexpected details request: %s", endpoint)
		}
		if test.failed {
			return errors.New("provider unavailable")
		}
		return json.Unmarshal([]byte(test.body), target)
	}}
	record := Record{Plot: "original", ProviderIDs: test.ids}
	poster := "original.jpg"
	provider.TVMetadata(t.Context(), library.Item{Season: 1, Episode: test.episode}, 3, "Series", "2020-01-01", &record, &poster)
	if !reflect.DeepEqual(record, test.want) || poster != test.poster {
		t.Fatalf("record=%#v poster=%q", record, poster)
	}
	wantCalls := 1
	if test.episode == 0 {
		wantCalls = 0
	}
	if calls != wantCalls {
		t.Fatalf("provider calls=%d, want %d", calls, wantCalls)
	}
}

func TestTMDBMovieCollection(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, body, want string
		failed           bool
	}{
		{"collection", `{"belongs_to_collection":{"name":"Trilogy"}}`, "Trilogy", false},
		{"missing", `{}`, "", false},
		{"null", `{"belongs_to_collection":null}`, "", false},
		{"unavailable", "", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := TMDBDetails{BaseURL: "https://provider.example///", Fetch: func(ctx context.Context, endpoint string, target any) error {
				if ctx != t.Context() || endpoint != "https://provider.example/movie/3" {
					t.Fatalf("unexpected collection request: %s", endpoint)
				}
				if test.failed {
					return errors.New("provider unavailable")
				}
				return json.Unmarshal([]byte(test.body), target)
			}}
			if got := provider.MovieCollection(t.Context(), 3); got != test.want {
				t.Fatalf("collection=%q, want %q", got, test.want)
			}
		})
	}
}
