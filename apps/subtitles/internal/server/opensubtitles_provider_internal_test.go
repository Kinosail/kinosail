package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestOpenSubtitlesSearchAndDownloadUseTrustedSession(t *testing.T) { //nolint:gocognit,cyclop,funlen // One integration fixture covers login, search, cache reuse, download, and host trust.
	t.Parallel()
	var loginCount, searchCount atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/subtitle.srt" && request.Header.Get("Api-Key") != "app-key" {
			http.Error(writer, "missing API key", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/subtitles":
			searches := searchCount.Add(1)
			if request.URL.Query().Get("imdb_id") != "2543164" || request.URL.Query().Get("languages") != "en" || request.URL.Query().Get("moviehash") == "" {
				http.Error(writer, "bad search", http.StatusBadRequest)
				return
			}
			if searches > 1 && request.Header.Get("Authorization") != "Bearer session-token" {
				http.Error(writer, "missing cached token", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(writer).Encode(openSubtitlesResponse{TotalCount: 1, Data: []openSubtitlesSearchItem{{
				ID: "1", Type: "subtitle", Attributes: openSubtitlesAttributes{
					Language: "en", FromTrusted: true, MoviehashMatch: true, Release: "other-release",
					Feature: openSubtitlesFeature{Type: "movie", Year: 2016, Title: "Arrival", IMDBID: 2543164},
					Files:   []openSubtitlesSubtitleFile{{FileID: 42, CDNumber: 1, Name: "Arrival.en.srt"}},
				},
			}}})
		case "/login":
			loginCount.Add(1)
			_ = json.NewEncoder(writer).Encode(openSubtitlesLogin{Status: http.StatusOK, Token: "session-token"})
		case "/download":
			if request.Header.Get("Authorization") != "Bearer session-token" {
				http.Error(writer, "missing token", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(writer).Encode(openSubtitlesDownload{Link: server.URL + "/subtitle.srt"})
		case "/subtitle.srt":
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)

	path := filepath.Join(t.TempDir(), "Arrival.2016.1080p.BluRay.mkv")
	if err := os.WriteFile(path, make([]byte, openSubtitlesHashBlock*2), 0o600); err != nil {
		t.Fatal(err)
	}
	provider := newOpenSubtitlesProvider(OpenSubtitlesConfig{URL: server.URL, APIKey: "app-key", Username: "owner", Password: "password"})
	item := library.Item{Kind: "video", Title: "Arrival", Year: "2016", Path: path, ProviderIDs: map[string]string{"imdb": "tt2543164"}}
	candidates, err := provider.search(context.Background(), item, "en")
	if err != nil || len(candidates) != 1 || candidates[0].FileID != 42 || candidates[0].ReleaseMatch != 1 {
		t.Fatalf("candidates = %#v, error = %v", candidates, err)
	}
	for range 2 {
		data, err := provider.download(context.Background(), candidates[0].FileID)
		if err != nil || len(data) == 0 {
			t.Fatalf("download = %q, error = %v", data, err)
		}
	}
	if _, err := provider.search(context.Background(), item, "en"); err != nil {
		t.Fatalf("search with cached session: %v", err)
	}
	if loginCount.Load() != 1 {
		t.Fatalf("login count = %d", loginCount.Load())
	}
}

func TestOpenSubtitlesFailedLoginBacksOffWithoutDownload(t *testing.T) {
	t.Parallel()
	var loginCount, downloadCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/login":
			loginCount.Add(1)
			http.Error(writer, "no", http.StatusUnauthorized)
		default:
			downloadCount.Add(1)
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	provider := newOpenSubtitlesProvider(OpenSubtitlesConfig{URL: server.URL, APIKey: "app-key", Username: "owner", Password: "password"})
	for range 2 {
		if data, err := provider.download(context.Background(), 42); err == nil || data != nil {
			t.Fatalf("download = %q, error = %v", data, err)
		}
	}
	if loginCount.Load() != 1 || downloadCount.Load() != 0 {
		t.Fatalf("login count = %d, download count = %d", loginCount.Load(), downloadCount.Load())
	}
}

func TestSubtitleProviderCredentialRejectsControls(t *testing.T) {
	t.Parallel()
	for name, value := range map[string]string{
		"line feed": "key\nvalue", "carriage return": "key\rvalue", "tab": "key\tvalue",
		"null": "key\x00value", "delete": "key\x7fvalue", "invalid utf8": string([]byte{0xff}),
	} {
		t.Run(name, func(t *testing.T) {
			if validSubtitleProviderCredential(value) {
				t.Fatal("unsafe provider credential was accepted")
			}
			if (&subtitleProvider{config: SubtitleConfig{URL: "https://api.subdl.com/api/v1", APIKey: value}}).subDLConfigured() {
				t.Fatal("SubDL reported an unsafe credential as configured")
			}
			if newOpenSubtitlesProvider(OpenSubtitlesConfig{URL: "https://api.opensubtitles.com/api/v1", APIKey: value, Username: "owner", Password: "password"}).configured() {
				t.Fatal("OpenSubtitles reported an unsafe credential as configured")
			}
			if newSubSourceProvider(SubSourceConfig{URL: "https://api.subsource.net/api/v1", APIKey: value, PersonalUse: true}).configured() {
				t.Fatal("SubSource reported an unsafe credential as configured")
			}
		})
	}
}

func TestRankOpenSubtitlesRejectsIdentityAndMachineConflicts(t *testing.T) {
	t.Parallel()
	item := library.Item{Title: "Arrival", Year: "2016", Path: "Arrival.2016.mkv", ProviderIDs: map[string]string{"imdb": "tt2543164"}}
	valid := openSubtitlesAttributes{
		Language: "en", Release: "Arrival.2016", Feature: openSubtitlesFeature{Type: "movie", Year: 2016, Title: "Arrival", IMDBID: 2543164},
		Files: []openSubtitlesSubtitleFile{{FileID: 42, CDNumber: 1, Name: "Arrival.en.srt"}},
	}
	wrongID, machine := valid, valid
	wrongID.Feature.IMDBID = 1
	machine.MachineTranslated = true
	response := openSubtitlesResponse{TotalCount: 3, Data: []openSubtitlesSearchItem{
		{ID: "valid", Type: "subtitle", Attributes: valid},
		{ID: "wrong", Type: "subtitle", Attributes: wrongID},
		{ID: "machine", Type: "subtitle", Attributes: machine},
	}}
	candidates := rankOpenSubtitlesCandidates(item, "en", response)
	if len(candidates) != 1 || candidates[0].FileID != 42 {
		t.Fatalf("candidates = %#v", candidates)
	}
}
