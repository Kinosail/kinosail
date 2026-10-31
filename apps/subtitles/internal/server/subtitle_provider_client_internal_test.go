package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func captureProviderLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &logs
}

func providerLogLines(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(logs.String()), "\n") {
		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) == nil && entry["operation"] == "subtitle.provider" {
			lines = append(lines, entry)
		}
	}
	return lines
}

func TestProviderRequestsAreLoggedWithoutSecretsOrURLs(t *testing.T) {
	logs := captureProviderLogs(t)
	var fail atomic.Bool
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			http.Error(writer, "maintenance", http.StatusServiceUnavailable)
			return
		}
		_, _ = writer.Write([]byte(`{"status":true,"subtitles":[]}`))
	}))
	t.Cleanup(remote.Close)
	provider := newSubDLProvider(remote.URL+"/api/v1", "secret-subdl-key", newSubtitleProviderHealthRegistry())
	item := library.Item{ID: "item", Kind: "video", Title: "Film", Path: "/media/Film.2020.1080p.WEB-DL.mkv"}
	if _, err := provider.candidates(t.Context(), item, "en"); err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	provider.client.health.reset("SubDL")
	if _, err := provider.candidates(t.Context(), item, "en"); err == nil {
		t.Fatal("a 503 search succeeded")
	}
	lines := providerLogLines(t, logs)
	if len(lines) != 2 {
		t.Fatalf("provider log lines = %v", lines)
	}
	if lines[0]["level"] != "DEBUG" || lines[0]["provider"] != "SubDL" || lines[0]["route"] != "search" || lines[0]["outcome"] != "ok" || lines[0]["status"] != float64(200) {
		t.Errorf("success log = %v", lines[0])
	}
	if lines[1]["level"] != "WARN" || lines[1]["route"] != "search" || lines[1]["outcome"] != "rejected" || lines[1]["status"] != float64(503) {
		t.Errorf("failure log = %v", lines[1])
	}
	for _, leak := range []string{"secret-subdl-key", remote.URL, "film_name", "maintenance"} {
		if strings.Contains(logs.String(), leak) {
			t.Errorf("provider logs contain %q", leak)
		}
	}
}

func TestAnOversizedFileDoesNotMarkTheProviderUnavailable(t *testing.T) {
	t.Parallel()
	oversized := bytes.Repeat([]byte("a"), (4<<20)+16)
	var base atomic.Pointer[string]
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/login":
			_, _ = writer.Write([]byte(`{"token":"session","status":200}`))
		case "/download":
			_, _ = writer.Write([]byte(`{"link":"` + *base.Load() + `/file.srt"}`))
		default:
			_, _ = writer.Write(oversized)
		}
	}))
	t.Cleanup(remote.Close)
	base.Store(&remote.URL)
	health := newSubtitleProviderHealthRegistry()
	subDL := newSubDLProvider(remote.URL+"/api/v1", "key", health)
	if _, err := subDL.download(t.Context(), "/file.srt"); err == nil {
		t.Fatal("an oversized SubDL file was accepted")
	}
	open := newOpenSubtitlesProvider(OpenSubtitlesConfig{URL: remote.URL, APIKey: "key", Username: "user", Password: "password"}, health)
	if _, err := open.download(t.Context(), 7); err == nil {
		t.Fatal("an oversized OpenSubtitles file was accepted")
	}
	for _, name := range []string{"SubDL", "OpenSubtitles"} {
		if err := health.before(name); err != nil {
			t.Errorf("%s is unavailable after one oversized file: %v", name, err)
		}
	}
}

func TestOpenSubtitlesFileHostsNeverReceiveTheAPIKey(t *testing.T) {
	t.Parallel()
	var leaked atomic.Bool
	var base atomic.Pointer[string]
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/login":
			_, _ = writer.Write([]byte(`{"token":"session","status":200}`))
		case "/download":
			_, _ = writer.Write([]byte(`{"link":"` + *base.Load() + `/file.srt"}`))
		default:
			leaked.Store(request.Header.Get("Api-Key") != "" || request.Header.Get("Authorization") != "")
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nLine\n"))
		}
	}))
	t.Cleanup(remote.Close)
	base.Store(&remote.URL)
	provider := newOpenSubtitlesProvider(OpenSubtitlesConfig{URL: remote.URL, APIKey: "key", Username: "user", Password: "password"}, newSubtitleProviderHealthRegistry())
	if _, err := provider.download(t.Context(), 7); err != nil {
		t.Fatal(err)
	}
	if leaked.Load() {
		t.Fatal("the file host received OpenSubtitles credentials")
	}
}

func TestEverySourceReportsItsOwnCredentialCheck(t *testing.T) {
	t.Parallel()
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v2/me", "/login":
			_, _ = writer.Write([]byte(`{"token":"session","status":200}`))
		default:
			http.Error(writer, "denied", http.StatusUnauthorized)
		}
	}))
	t.Cleanup(remote.Close)
	provider := newSubtitleProvider(SubtitleConfig{
		URL: remote.URL + "/api/v1", APIKey: "key",
		OpenSubtitles: OpenSubtitlesConfig{URL: remote.URL, APIKey: "key", Username: "user", Password: "password"},
		SubSource:     SubSourceConfig{URL: remote.URL + "/subsource", APIKey: "key", PersonalUse: true},
	}, t.TempDir(), t.TempDir(), nil, nil, "")
	attempted, connected := provider.testCredentials(t.Context())
	if attempted != 3 || connected != 2 {
		t.Fatalf("credential checks = %d attempted, %d connected", attempted, connected)
	}
	states := map[string]string{}
	for _, view := range provider.healthViews() {
		states[view.Name] = view.State
	}
	if states["SubDL"] != "connected" || states["OpenSubtitles"] != "connected" || states["SubSource"] != "unavailable" {
		t.Fatalf("provider states = %v", states)
	}
}
