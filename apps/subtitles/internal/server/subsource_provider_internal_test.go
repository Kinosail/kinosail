package server

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubSourceSearchAndAcquisitionPreserveAcceptedSubtitle(t *testing.T) {
	t.Parallel()
	original := []byte("1\r\n00:00:01,000 --> 00:00:02,000\r\nSubtitle by Example\r\n\r\n2\r\n00:00:03,000 --> 00:00:04,000\r\nHello\r\n")
	remote := newSubSourceTestServer(t, original)
	item := library.Item{ID: "arrival", Kind: "video", Path: "/media/Arrival.2016.1080p.WEB-DL.mkv", Title: "Arrival", Year: "2016", ProviderIDs: map[string]string{"imdb": "tt2543164", "tmdb": "329865"}}
	config := SubtitleConfig{SubSource: SubSourceConfig{URL: remote.URL + "/api/v1", APIKey: "key", PersonalUse: true}}
	provider := newSubtitleProvider(config, t.TempDir(), t.TempDir(), nil, nil, "ffmpeg")
	cleaned, record, err := provider.acquire(context.Background(), item, "en", nil)
	if err != nil {
		t.Fatalf("acquire SubSource subtitle: %v", err)
	}
	if !bytes.Equal(cleaned.Data, original) || record.Source != "subsource" {
		t.Fatalf("SubSource content changed or provenance was lost: source=%q data=%q", record.Source, cleaned.Data)
	}
}

func TestSubSourceRejectsUnsafeOrAmbiguousResults(t *testing.T) {
	t.Parallel()
	item := library.Item{Path: "/media/Show.S01E02.mkv", Title: "Second", Show: "Show", ShowYear: "2024", Season: 1, Episode: 2, ShowProviderIDs: map[string]string{"imdb": "tt1234567"}}
	movie := subSourceMovie{MovieID: 7, Title: "Show", Type: "tvseries", ReleaseYear: 2024, IMDBID: "tt1234567", Season: 1, SubtitleCount: 4}
	valid := subSourceSubtitle{SubtitleID: 1, MovieID: 7, Language: "english", ReleaseInfo: []string{"Show.S01E02.WEB-DL"}, Files: 1, Size: 100}
	valid.Rating.Good, valid.Rating.Total = 4, 4
	for name, mutate := range map[string]func(*subSourceSubtitle){
		"wrong movie":            func(value *subSourceSubtitle) { value.MovieID = 8 },
		"wrong language":         func(value *subSourceSubtitle) { value.Language = "spanish" },
		"wrong episode":          func(value *subSourceSubtitle) { value.ReleaseInfo = []string{"Show.S01E03.WEB-DL"} },
		"machine translation":    func(value *subSourceSubtitle) { value.ProductionType = "machine" },
		"forced track":           func(value *subSourceSubtitle) { value.ProductionType = "forced" },
		"foreign parts":          func(value *subSourceSubtitle) { value.ForeignParts = true },
		"too many archive files": func(value *subSourceSubtitle) { value.Files = 101 },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			response := subSourceSubtitleResponse{Success: true, Data: []subSourceSubtitle{candidate}}
			response.Pagination.Limit, response.Pagination.Total = 50, 1
			if got := rankSubSourceCandidates(item, movie, "english", response); len(got) != 0 {
				t.Fatalf("unsafe result produced %d candidates", len(got))
			}
		})
	}
	response := subSourceSubtitleResponse{Success: true, Data: make([]subSourceSubtitle, 51)}
	response.Pagination.Limit, response.Pagination.Total = 100, 51
	if got := rankSubSourceCandidates(item, movie, "english", response); got != nil {
		t.Fatal("oversized provider result was accepted")
	}
}

func TestSubSourceArchiveNeedsOneExactEpisodeSRT(t *testing.T) {
	t.Parallel()
	item := library.Item{Show: "Show", Season: 1, Episode: 2}
	valid := []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n")
	for name, files := range map[string]map[string][]byte{
		"valid":     {"Show.S01E01.srt": valid, "Show.S01E02.srt": valid},
		"ambiguous": {"Show.S01E02.srt": valid, "Alt.S01E02.srt": valid},
		"wrong":     {"Show.S01E03.srt": valid},
		"not SRT":   {"Show.S01E02.vtt": valid},
	} {
		t.Run(name, func(t *testing.T) {
			archiveData := zipSubSourceFiles(t, files)
			archive, err := zip.NewReader(bytes.NewReader(archiveData), int64(len(archiveData)))
			if err != nil {
				t.Fatal(err)
			}
			got, err := readSubSourceArchive(archive, item)
			if (name == "valid") != (err == nil) {
				t.Fatalf("unexpected result: data=%q error=%v", got, err)
			}
		})
	}
}

func TestSubSourceRejectsRedirectOutsideConfiguredOrigin(t *testing.T) {
	t.Parallel()
	outside := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer outside.Close()
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", outside.URL)
		writer.WriteHeader(http.StatusFound)
	}))
	defer remote.Close()
	provider := newSubSourceProvider(SubSourceConfig{URL: remote.URL, APIKey: "key", PersonalUse: true})
	_, err := provider.download(context.Background(), subSourceCandidate{ID: 1}, library.Item{})
	if err == nil {
		t.Fatal("cross-origin redirect was accepted")
	}
}

func newSubSourceTestServer(t *testing.T, subtitle []byte) *httptest.Server { //nolint:cyclop // The fixture serves each provider endpoint contract.
	t.Helper()
	archive := zipSubSourceFiles(t, map[string][]byte{"Arrival.2016.1080p.WEB-DL.srt": subtitle})
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-API-Key") != "key" || request.Header.Get("User-Agent") == "" {
			t.Errorf("missing SubSource request headers")
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/api/v1/movies/search":
			if request.URL.Query().Get("searchType") != "imdb" || request.URL.Query().Get("imdb") != "tt2543164" {
				t.Errorf("unexpected movie query: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"success":true,"data":[{"movieId":7,"title":"Arrival","type":"movie","releaseYear":2016,"imdbId":"tt2543164","tmdbId":"329865","subtitleCount":1}]}`))
		case "/api/v1/subtitles":
			if request.URL.Query().Get("movieId") != "7" || request.URL.Query().Get("language") != "english" || request.URL.Query().Get("sort") != "rating" {
				t.Errorf("unexpected subtitle query: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"success":true,"data":[{"subtitleId":9,"movieId":7,"language":"english","releaseInfo":["Arrival.2016.1080p.WEB-DL"],"files":1,"size":128,"hearingImpaired":false,"foreignParts":false,"productionType":"retail","downloads":10,"rating":{"good":4,"bad":0,"total":4}}],"pagination":{"page":1,"limit":50,"total":1,"pages":1}}`))
		case "/api/v1/subtitles/9/download":
			writer.Header().Set("Content-Type", "application/zip")
			_, _ = writer.Write(archive)
		default:
			http.NotFound(writer, request)
		}
	}))
}

func zipSubSourceFiles(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for name, data := range files {
		writer, err := archive.Create(filepath.ToSlash(name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestSubSourceLanguageMappingIsBounded(t *testing.T) {
	t.Parallel()
	for _, language := range []string{"en", "EN", "pt-br", "ja", "zh"} {
		if _, ok := subSourceLanguage(language); !ok {
			t.Fatalf("supported language %q was rejected", language)
		}
	}
	for _, language := range []string{"", strings.Repeat("x", 200)} {
		if _, ok := subSourceLanguage(language); ok {
			t.Fatalf("unsupported language %q was accepted", language)
		}
	}
}
