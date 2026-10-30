package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func openSubtitlesQuotaFixture(t *testing.T, handle func(http.ResponseWriter, *http.Request) bool) *openSubtitlesProvider {
	t.Helper()
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if handle(writer, request) {
			return
		}
		if request.URL.Path == "/login" {
			_ = json.NewEncoder(writer).Encode(map[string]any{"token": "session", "status": 200})
			return
		}
		http.NotFound(writer, request)
	}))
	t.Cleanup(remote.Close)
	return newOpenSubtitlesProvider(OpenSubtitlesConfig{URL: remote.URL, APIKey: "key", Username: "user", Password: "password"})
}

func TestOpenSubtitlesDailyDownloadQuotaWaitsForTheReset(t *testing.T) {
	t.Parallel()
	provider := openSubtitlesQuotaFixture(t, func(writer http.ResponseWriter, request *http.Request) bool {
		if request.URL.Path != "/download" {
			return false
		}
		writer.WriteHeader(http.StatusNotAcceptable)
		_, _ = writer.Write([]byte(`{"message":"You have downloaded your allowed 20 subtitles for 24h"}`))
		return true
	})
	if _, err := provider.download(t.Context(), 7); err == nil {
		t.Fatal("quota response produced a download")
	}
	views := provider.health.views(map[string]bool{"OpenSubtitles": true})
	var state, retry string
	for _, view := range views {
		if view.Name == "OpenSubtitles" {
			state, retry = view.State, view.NextRetry
		}
	}
	next, err := time.Parse(time.RFC3339, retry)
	if state != "limited" || err != nil || time.Until(next) < 30*time.Minute && next.UTC().Hour() != 0 {
		t.Fatalf("quota state = %q until %q (%v)", state, retry, err)
	}
	midnight := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	if next.After(midnight.Add(time.Minute)) || next.Before(time.Now()) {
		t.Fatalf("quota retry %v is not at the next UTC reset %v", next, midnight)
	}
}

func TestOpenSubtitlesStaleTokenIsReplacedInsteadOfBlockingForADay(t *testing.T) {
	t.Parallel()
	var searches atomic.Int64
	provider := openSubtitlesQuotaFixture(t, func(writer http.ResponseWriter, request *http.Request) bool {
		if request.URL.Path != "/download" {
			return false
		}
		searches.Add(1)
		writer.WriteHeader(http.StatusUnauthorized)
		return true
	})
	provider.token, provider.sessionURL, provider.tokenExpires = "stale", provider.config.URL, time.Now().Add(time.Hour)
	if _, err := provider.download(t.Context(), 7); err == nil {
		t.Fatal("unauthorized response produced a download")
	}
	provider.mu.Lock()
	token := provider.token
	provider.mu.Unlock()
	if token != "" {
		t.Fatalf("stale token kept after 401: %q", token)
	}
	for _, view := range provider.health.views(map[string]bool{"OpenSubtitles": true}) {
		if view.Name != "OpenSubtitles" || view.NextRetry == "" {
			continue
		}
		if next, err := time.Parse(time.RFC3339, view.NextRetry); err != nil || time.Until(next) > time.Hour {
			t.Fatalf("stale token blocked the provider until %q", view.NextRetry)
		}
	}
}
