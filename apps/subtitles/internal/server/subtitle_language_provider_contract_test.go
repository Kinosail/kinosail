package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestCriticalVariantsUseExactOpenSubtitlesCodes(t *testing.T) { //nolint:cyclop,gocognit // Exact provider mappings are checked as one contract.
	tests := []struct{ tag, code string }{{"pt-BR", "pt-br"}, {"pt-PT", "pt-pt"}, {"zh-Hans", "zh-cn"}, {"zh-Hant", "zh-tw"}, {"es-419", "ea"}}
	seen := make([]string, 0, len(tests))
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		language := request.URL.Query().Get("languages")
		seen = append(seen, language)
		if language == "pt" || language == "zh" || language == "es" {
			t.Errorf("exact variant broadened to %q", language)
		}
		_ = json.NewEncoder(writer).Encode(openSubtitlesResponse{})
	}))
	t.Cleanup(remote.Close)
	provider := newOpenSubtitlesProvider(OpenSubtitlesConfig{URL: remote.URL, APIKey: "key", Username: "owner", Password: "password"})
	item := library.Item{Path: "/media/Movie.mkv", Title: "Movie"}
	for _, test := range tests {
		before := len(seen)
		if _, err := provider.search(context.Background(), item, test.tag); err != nil {
			t.Errorf("search %s: %v", test.tag, err)
		} else if len(seen) != before+1 || seen[before] != test.code {
			t.Errorf("search %s sent %q; want %q", test.tag, seen[before:], test.code)
		}
	}
	before := len(seen)
	if _, err := provider.search(context.Background(), item, "sr-Latn"); err == nil || len(seen) != before {
		t.Fatalf("unsupported sr-Latn: error=%v calls=%d", err, len(seen)-before)
	}
}

func TestCriticalVariantsUseOnlyExactSubDLAndSubSourceCodes(t *testing.T) { //nolint:cyclop,gocognit // Exact provider mappings are checked as one contract.
	item := library.Item{Path: "/media/Movie.mkv", Title: "Movie", Year: "2024", ProviderIDs: map[string]string{"imdb": "tt1234567"}}
	subDLSeen := make([]string, 0, 2)
	subDLRemote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		language := request.URL.Query().Get("languages")
		subDLSeen = append(subDLSeen, language)
		if language != "BR_PT" && language != "ZH_BG" {
			t.Errorf("unexpected SubDL language %q", language)
		}
		_ = json.NewEncoder(writer).Encode(subDLResponse{Status: true})
	}))
	t.Cleanup(subDLRemote.Close)
	subDL := newSubtitleProvider(SubtitleConfig{URL: subDLRemote.URL, APIKey: "key"}, t.TempDir(), t.TempDir(), nil, nil, "")
	for index, test := range []struct{ tag, code string }{{"pt-BR", "BR_PT"}, {"zh-Hant", "ZH_BG"}} {
		if _, err := subDL.searchSubDL(context.Background(), item, test.tag); err != nil {
			t.Errorf("SubDL search %s: %v", test.tag, err)
		} else if len(subDLSeen) != index+1 || subDLSeen[index] != test.code {
			t.Errorf("SubDL search %s sent %q; want %q", test.tag, subDLSeen[index:], test.code)
		}
	}
	for _, tag := range []string{"pt-PT", "zh-Hans", "es-419", "sr-Latn"} {
		before := len(subDLSeen)
		if _, err := subDL.searchSubDL(context.Background(), item, tag); err == nil || len(subDLSeen) != before {
			t.Errorf("unsupported SubDL %s: error=%v calls=%d", tag, err, len(subDLSeen)-before)
		}
	}

	subSourceCalls := 0
	subSourceRemote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		subSourceCalls++
		switch request.URL.Path {
		case "/movies/search":
			_, _ = writer.Write([]byte(`{"success":true,"data":[{"movieId":7,"title":"Movie","type":"movie","releaseYear":2024,"imdbId":"tt1234567","subtitleCount":1}]}`))
		case "/subtitles":
			if language := request.URL.Query().Get("language"); language != "brazilian_portuguese" {
				t.Errorf("unexpected SubSource language %q", language)
			}
			_, _ = writer.Write([]byte(`{"success":true,"data":[],"pagination":{"page":1,"limit":50,"total":0,"pages":0}}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(subSourceRemote.Close)
	subSource := newSubSourceProvider(SubSourceConfig{URL: subSourceRemote.URL, APIKey: "key", PersonalUse: true})
	if _, err := subSource.search(context.Background(), item, "pt-BR"); err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"pt-PT", "zh-Hans", "zh-Hant", "es-419", "sr-Latn"} {
		before := subSourceCalls
		if _, err := subSource.search(context.Background(), item, tag); err == nil || subSourceCalls != before {
			t.Errorf("unsupported SubSource %s: error=%v calls=%d", tag, err, subSourceCalls-before)
		}
	}
}
