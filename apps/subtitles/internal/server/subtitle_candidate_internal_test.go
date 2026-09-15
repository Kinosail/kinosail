package server

import (
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/subtitlelanguage"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestRankSubDLCandidatesUsesIdentityReleaseAndAccessibilityEvidence(t *testing.T) {
	t.Parallel()
	item := library.Item{Title: "Arrival", Year: "2016", Path: "/media/Arrival.2016.1080p.BluRay.x264-GROUP.mkv", ProviderIDs: map[string]string{"imdb": "tt2543164", "tmdb": "329865"}}
	response := subDLResponse{Status: true, Results: []subDLResult{{IMDBID: "tt2543164", TMDBID: 329865, Type: "movie", Name: "Arrival", Year: 2016}}, Subtitles: []subDLSubtitle{
		{URL: "/machine.srt", ReleaseName: "Arrival 2016 GROUP", Language: "EN", ProductionType: 3},
		{URL: "/hi.srt", ReleaseName: "Arrival 2016 GROUP", Language: "EN", HI: true},
		{URL: "/best.srt", ReleaseName: "Arrival.2016.1080p.BluRay.x264-GROUP", Language: "EN"},
		{URL: "/wrong-language.srt", ReleaseName: "Arrival GROUP", Language: "FR"},
		{URL: "/missing-language.srt", ReleaseName: "Arrival GROUP"},
		{URL: "/unknown-language.srt", ReleaseName: "Arrival GROUP", Language: "xx-US"},
	}}
	candidates := rankSubDLCandidates(item, "en", response)
	if len(candidates) != 3 || candidates[0].URL != "/best.srt" || candidates[0].Score != 100 || candidates[2].Machine != true {
		t.Fatalf("candidates = %#v", candidates)
	}
}

func TestRankSubDLCandidatesRejectsIdentityConflictsAndSelectsEpisodeFromPack(t *testing.T) {
	t.Parallel()
	item := library.Item{Show: "Severance", ShowYear: "2022", Season: 1, Episode: 2, Path: "/media/Severance.S01E02.mkv", ShowProviderIDs: map[string]string{"imdb": "tt11280740"}}
	pack := subDLSubtitle{FullSeason: true, UnpackFiles: []subDLFile{{URL: "/one.srt", ReleaseName: "Severance S01E01", Language: "EN", Season: 1, Episode: 1}, {URL: "/two.srt", ReleaseName: "Severance S01E02", Language: "EN", Season: 1, Episode: 2}}}
	valid := rankSubDLCandidates(item, "en", subDLResponse{Status: true, Results: []subDLResult{{IMDBID: "tt11280740", Type: "tv", Name: "Severance", Year: 2022}}, Subtitles: []subDLSubtitle{pack}})
	wrong := rankSubDLCandidates(item, "en", subDLResponse{Status: true, Results: []subDLResult{{IMDBID: "tt0000000", Type: "tv", Name: "Severance", Year: 2022}}, Subtitles: []subDLSubtitle{pack}})
	if len(valid) != 1 || valid[0].URL != "/two.srt" || valid[0].Episode != 2 || len(wrong) != 0 {
		t.Fatalf("valid = %#v, wrong = %#v", valid, wrong)
	}
}

func TestSubtitleLanguageSupportsCommonRegionalCodes(t *testing.T) {
	t.Parallel()
	for _, language := range []string{"en", "EN", "fil", "pt-br", "PT-br", "zh-cn", "ZH-hant"} {
		if !validLanguage(language) {
			t.Fatalf("rejected %q", language)
		}
	}
	for _, language := range []string{"e", "pt_BR", "abcd", "en-US", "en-us-extra"} {
		if validLanguage(language) {
			t.Fatalf("accepted %q", language)
		}
	}
}

func TestSubtitleLanguageMatchingAllowsOnlyRequestedBaseFallback(t *testing.T) {
	t.Parallel()
	for _, languages := range [][2]string{{"pt", "pt-pt"}, {"zh", "zh-cn"}, {"tl", "TL"}, {"pt-BR", "pt-br"}, {"zh-Hant", "zh-tw"}, {"en", "en-US"}} {
		if !subtitleLanguageMatches(languages[0], languages[1]) {
			t.Errorf("%q did not match %q", languages[0], languages[1])
		}
	}
	for _, languages := range [][2]string{{"pt-BR", "pt-pt"}, {"zh-Hant", "zh-cn"}, {"es-419", "es"}} {
		if subtitleLanguageMatches(languages[0], languages[1]) {
			t.Errorf("%q broadly matched %q", languages[0], languages[1])
		}
	}
}

func TestAdvertisedProviderCodesMatchTheirCatalogLanguage(t *testing.T) {
	t.Parallel()
	for _, choice := range subtitlelanguage.Catalog() {
		for provider, supported := range choice.Providers {
			if !supported {
				continue
			}
			code, ok := subtitlelanguage.Code(choice.Tag, provider)
			if !ok || !subtitleProviderLanguageMatches(choice.Tag, code, provider) {
				t.Errorf("%s advertises %s as %q, which does not match", provider, choice.Tag, code)
			}
		}
	}
}
