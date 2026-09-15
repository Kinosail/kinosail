package server

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestOpenSubtitlesCandidateBoundariesAndOrdering(t *testing.T) {
	t.Parallel()
	item := library.Item{Title: "Arrival", Year: "2016", Path: "Arrival.2016.mkv", ProviderIDs: map[string]string{"imdb": "tt2543164"}}
	valid := openSubtitlesAttributes{
		Language: "en",
		Release:  "Arrival.2016",
		Feature:  openSubtitlesFeature{Type: "movie", Year: 2016, Title: "Arrival", IMDBID: 2543164},
		Files:    []openSubtitlesSubtitleFile{{FileID: 1, CDNumber: 1, Name: "Arrival.en.srt"}},
	}
	if candidates := rankOpenSubtitlesCandidates(item, "en", openSubtitlesResponse{}); candidates != nil {
		t.Fatal("empty OpenSubtitles response was accepted")
	}
	wrongType, wrongFile := valid, valid
	wrongType.Files = valid.Files
	wrongFile.Files = []openSubtitlesSubtitleFile{{FileID: 0, Name: "Arrival.en.srt"}}
	invalid := openSubtitlesResponse{TotalCount: 2, Data: []openSubtitlesSearchItem{
		{Type: "movie", Attributes: wrongType},
		{Type: "subtitle", Attributes: wrongFile},
	}}
	if candidates := rankOpenSubtitlesCandidates(item, "en", invalid); len(candidates) != 0 {
		t.Fatalf("invalid OpenSubtitles candidates = %#v", candidates)
	}

	attributes := make([]openSubtitlesAttributes, 4)
	for index := range attributes {
		attributes[index] = valid
		attributes[index].Files = []openSubtitlesSubtitleFile{{FileID: int64(index + 1), CDNumber: 1, Name: "Arrival.en.srt"}}
	}
	attributes[0].HearingImpaired = true
	attributes[1].Release = "Arrival"
	attributes[2].FromTrusted = true
	response := openSubtitlesResponse{TotalCount: len(attributes)}
	for index := range attributes {
		response.Data = append(response.Data, openSubtitlesSearchItem{Type: "subtitle", Attributes: attributes[index]})
	}
	candidates := rankOpenSubtitlesCandidates(item, "en", response)
	if len(candidates) != maximumDownloadTries || candidates[0].FileID != 3 || candidates[2].FileID != 1 {
		t.Fatalf("ordered OpenSubtitles candidates = %#v", candidates)
	}
	equalHigh, equalLow := valid, valid
	equalHigh.AITranslated, equalHigh.HearingImpaired = true, true
	equalLow.AITranslated, equalLow.FromTrusted, equalLow.Release = true, true, "Arrival"
	equalHigh.Files = []openSubtitlesSubtitleFile{{FileID: 10, Name: "Arrival.en.srt"}}
	equalLow.Files = []openSubtitlesSubtitleFile{{FileID: 11, Name: "Arrival.en.srt"}}
	equal := rankOpenSubtitlesCandidates(item, "en", openSubtitlesResponse{TotalCount: 2, Data: []openSubtitlesSearchItem{{Type: "subtitle", Attributes: equalLow}, {Type: "subtitle", Attributes: equalHigh}}})
	if len(equal) != 2 || equal[0].FileID != 10 {
		t.Fatalf("equal-score OpenSubtitles candidates = %#v", equal)
	}
}

func TestOpenSubtitlesCandidateIdentityAndQualityBranches(t *testing.T) { //nolint:funlen // One table covers independent provider identity boundaries.
	t.Parallel()
	movie := library.Item{Title: "Arrival", Year: "2016", Path: "Arrival.2016.mkv", ProviderIDs: map[string]string{"imdb": "tt2543164"}}
	valid := openSubtitlesAttributes{Language: "en", Release: "Arrival.2016", Feature: openSubtitlesFeature{Type: "movie", Year: 2016, Title: "Arrival", IMDBID: 2543164}}
	episodeItem := library.Item{Show: "Example", ShowYear: "2025", Season: 1, Episode: 2, Path: "Example.S01E02.mkv", ShowProviderIDs: map[string]string{"imdb": "tt123", "tmdb": "456"}}
	episode := openSubtitlesAttributes{Language: "en", Release: "Example.S01E02", Feature: openSubtitlesFeature{Type: "episode", Season: 1, Episode: 2, ParentTitle: "Example", ParentIMDBID: 123, ParentTMDBID: 456, Year: 2025}}
	if _, _, ok := scoreOpenSubtitlesCandidate(episodeItem, "en", episode, "Example.S01E02.srt"); !ok {
		t.Fatal("valid episode identity was rejected")
	}
	wrongEpisode := episode
	wrongEpisode.Feature.Episode = 3
	if _, _, ok := scoreOpenSubtitlesCandidate(episodeItem, "en", wrongEpisode, "Example.S01E03.srt"); ok {
		t.Fatal("wrong episode identity was accepted")
	}
	wrongType := valid
	wrongType.Feature.Type = "episode"
	if _, _, ok := scoreOpenSubtitlesCandidate(movie, "en", wrongType, "Arrival.en.srt"); ok {
		t.Fatal("episode result was accepted for a movie")
	}

	for name, test := range map[string]struct {
		item       library.Item
		attributes openSubtitlesAttributes
		valid      bool
	}{
		"wrong TMDB": {
			library.Item{Title: "Arrival", Path: movie.Path, ProviderIDs: map[string]string{"tmdb": "99"}},
			openSubtitlesAttributes{Language: "en", Feature: openSubtitlesFeature{Type: "movie", Title: "Arrival", TMDBID: 100}},
			false,
		},
		"TMDB anchor": {
			library.Item{Title: "Arrival", Path: movie.Path, ProviderIDs: map[string]string{"tmdb": "100"}},
			openSubtitlesAttributes{Language: "en", Feature: openSubtitlesFeature{Type: "movie", Title: "Arrival", TMDBID: 100}},
			true,
		},
		"weak title": {
			library.Item{Title: "Arrival", Path: movie.Path},
			openSubtitlesAttributes{Language: "en", Feature: openSubtitlesFeature{Type: "movie", Title: "Different"}},
			false,
		},
		"wrong year": {
			movie,
			openSubtitlesAttributes{Language: "en", Feature: openSubtitlesFeature{Type: "movie", Title: "Arrival", IMDBID: 2543164, Year: 2015}},
			false,
		},
		"AI translation": {
			movie,
			openSubtitlesAttributes{Language: "en", AITranslated: true, Feature: valid.Feature},
			true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, _, ok := scoreOpenSubtitlesCandidate(test.item, "en", test.attributes, "Arrival.en.srt")
			if ok != test.valid {
				t.Fatalf("candidate validity = %v, want %v", ok, test.valid)
			}
		})
	}
	if numericProviderID("12x") != "" {
		t.Fatal("nonnumeric provider ID was accepted")
	}
}

func TestSubSourceMovieAndSubtitleBoundaryBranches(t *testing.T) { //nolint:funlen // One fixture covers provider movie and subtitle boundaries.
	t.Parallel()
	item := library.Item{Title: "Arrival", Year: "2016", Path: "Arrival.2016.mkv", ProviderIDs: map[string]string{"imdb": "tt2543164", "tmdb": "329865"}}
	if _, ok := selectSubSourceMovie(item, item.ProviderIDs, item.Title, item.Year, subSourceMovieResponse{}); ok {
		t.Fatal("empty SubSource movie response was accepted")
	}
	movies := []subSourceMovie{
		{MovieID: 0, Title: "Arrival"},
		{MovieID: 1, Title: "Arrival", Type: "tvseries"},
		{MovieID: 2, Title: "Arrival", Type: "movie", IMDBID: "tt1"},
		{MovieID: 3, Title: "Arrival", Type: "movie", IMDBID: "tt2543164", TMDBID: "1"},
		{MovieID: 4, Title: "Arrival", Type: "movie", ReleaseYear: 2015, IMDBID: "tt2543164", TMDBID: "329865"},
		{MovieID: 5, Title: "Arrival", Type: "movie", ReleaseYear: 2016, IMDBID: "tt2543164", TMDBID: "329865"},
	}
	movie, ok := selectSubSourceMovie(item, item.ProviderIDs, item.Title, item.Year, subSourceMovieResponse{Success: true, Data: movies})
	if !ok || movie.MovieID != 5 {
		t.Fatalf("selected SubSource movie = %#v, valid %v", movie, ok)
	}

	show := library.Item{Show: "Example", Season: 1, Episode: 2, Path: "Example.S01E02.mkv"}
	selected := subSourceMovie{MovieID: 5}
	invalid := subSourceSubtitle{SubtitleID: 0, MovieID: 5, Language: "english", ReleaseInfo: []string{"Example.S01E02"}, Files: 1, Size: 100}
	longRelease := subSourceSubtitle{SubtitleID: 1, MovieID: 5, Language: "english", ReleaseInfo: []string{strings.Repeat("x", 1100), strings.Repeat("y", 1100)}, Files: 1, Size: 100}
	multi := subSourceSubtitle{SubtitleID: 2, MovieID: 5, Language: "english", ReleaseInfo: []string{"Example season pack"}, Files: 2, Size: 100}
	multi.Rating.Good, multi.Rating.Total = 2, 2
	second := multi
	second.SubtitleID = 3
	second.ReleaseInfo = []string{"Example S01E02"}
	response := subSourceSubtitleResponse{Success: true, Data: []subSourceSubtitle{invalid, longRelease, multi, second}}
	response.Pagination.Page, response.Pagination.Limit, response.Pagination.Total, response.Pagination.Pages = 1, 50, 4, 1
	candidates := rankSubSourceCandidates(show, selected, "en", response)
	if len(candidates) != 2 || candidates[0].ID != 3 || candidates[1].ReleaseMatch < 0.8 {
		t.Fatalf("SubSource candidates = %#v", candidates)
	}
}

func TestSubDLCandidateBoundaryBranches(t *testing.T) { //nolint:cyclop,funlen // One fixture covers strict candidate filtering and ranking.
	t.Parallel()
	item := library.Item{Title: "Arrival", Year: "2016", Path: "Arrival.2016.mkv"}
	if candidates := rankSubDLCandidates(item, "en", subDLResponse{}); candidates != nil {
		t.Fatal("empty SubDL response was accepted")
	}
	tooManyFiles := subDLSubtitle{UnpackFiles: make([]subDLFile, 101)}
	response := subDLResponse{Status: true, Results: []subDLResult{{Type: "movie", Name: "Arrival", Year: 2016}}, Subtitles: []subDLSubtitle{tooManyFiles}}
	if candidates := rankSubDLCandidates(item, "en", response); candidates != nil {
		t.Fatal("oversized SubDL archive was accepted")
	}
	response.Results = []subDLResult{{Type: "tv", Name: "Arrival", Year: 2016}}
	response.Subtitles = []subDLSubtitle{{URL: "/arrival.srt", Language: "en"}}
	if candidates := rankSubDLCandidates(item, "en", response); candidates != nil {
		t.Fatal("SubDL series result was accepted for a movie")
	}

	result := subDLResult{Type: "movie", Name: "Arrival", Year: 2016}
	response.Subtitles = []subDLSubtitle{
		{URL: "/one.srt", ReleaseName: "Arrival.2016", Language: "en"},
		{URL: "/two.srt", ReleaseName: "Arrival", Language: "en"},
		{URL: "/three.srt", ReleaseName: "Arrival.2016", Language: "en", HI: true},
		{URL: "/four.srt", ReleaseName: "Arrival.2016", Language: "en"},
		{URL: "/five.srt", ReleaseName: "Arrival.2016", Language: "en", HI: true},
		{URL: "/machine.srt", ReleaseName: "Arrival.2016", Language: "en", ProductionType: 3},
		{URL: "/pack.zip", FullSeason: true, Language: "en"},
	}
	response.Results = []subDLResult{result}
	candidates := rankSubDLCandidates(item, "en", response)
	if len(candidates) != maximumDownloadTries || candidates[0].URL != "/one.srt" {
		t.Fatalf("ranked SubDL candidates = %#v", candidates)
	}

	for name, test := range map[string]struct {
		item      library.Item
		result    subDLResult
		candidate subtitleCandidate
	}{
		"empty URL":     {item, result, subtitleCandidate{}},
		"invalid URL":   {item, result, subtitleCandidate{URL: "%", Language: "en"}},
		"weak title":    {item, subDLResult{Name: "Different"}, subtitleCandidate{URL: "/a.srt", Language: "en"}},
		"wrong year":    {item, subDLResult{Name: "Arrival", Year: 2015}, subtitleCandidate{URL: "/a.srt", Language: "en"}},
		"wrong episode": {library.Item{Show: "Example", Season: 1, Episode: 2}, subDLResult{Type: "tv", Name: "Example"}, subtitleCandidate{URL: "/a.srt", Language: "en", Season: 1, Episode: 3}},
		"wrong TMDB":    {library.Item{Title: "Arrival", ProviderIDs: map[string]string{"tmdb": "1"}}, subDLResult{Name: "Arrival", TMDBID: 2}, subtitleCandidate{URL: "/a.srt", Language: "en"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, _, ok := scoreSubtitleCandidate(test.item, "en", test.result, test.candidate); ok {
				t.Fatal("invalid SubDL candidate was accepted")
			}
		})
	}
	if canonicalSubtitleLanguage("invalid") != "" || subDLIdentityMatches(library.Item{Show: "Example"}, subDLResult{Type: "movie"}) || firstNonempty("", "") != "" {
		t.Fatal("invalid SubDL helper boundary was accepted")
	}
	tmdbItem := library.Item{Title: "Arrival", ProviderIDs: map[string]string{"tmdb": "2"}}
	if _, _, ok := scoreSubtitleCandidate(tmdbItem, "en", subDLResult{Name: "Arrival", TMDBID: 2}, subtitleCandidate{URL: "/arrival.srt", Language: "en"}); !ok {
		t.Fatal("valid TMDB-only SubDL candidate was rejected")
	}
}
