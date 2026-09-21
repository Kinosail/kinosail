package servertest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func (fixture AutomaticSkipFixture) BundledPlayerKeepsDirectTimelineAndClientSkip(t *testing.T) {
	t.Helper()
	handler, token, _, id, _, _ := fixture.server(t)
	player := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(player, request)
	for _, expected := range []string{`src="/media/`, `data-adaptive="/hls/`, `data-auto-skip="intro"`, `data-duration="60"`, `data-marker="intro"`} {
		if player.Code != http.StatusOK || !strings.Contains(player.Body.String(), expected) {
			t.Fatalf("direct player lacks %q: %d %q", expected, player.Code, player.Body.String())
		}
	}
	if strings.Contains(player.Body.String(), `data-hls=`) || strings.Contains(player.Body.String(), `?playbackToken=`) {
		t.Fatalf("direct player starts a transformed timeline: %q", player.Body.String())
	}
	assertAutomaticSkipSubtitle(t, handler, token, player.Body.String())
	progressPath := regexp.MustCompile(`data-progress="([^"]+)"`).FindStringSubmatch(player.Body.String())
	if len(progressPath) != 2 {
		t.Fatalf("chopped player progress URL = %q", player.Body.String())
	}
	progress := httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, progressPath[1], strings.NewReader(url.Values{"seconds": {"15"}}.Encode()))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(progress, request)
	source := APICall(t, handler, token, http.MethodGet, "/api/v1/items/"+id, nil)
	if progress.Code != http.StatusNoContent || !strings.Contains(source.Body.String(), `"seconds":15`) {
		t.Fatalf("bundled progress = %d %q, source = %d %q", progress.Code, progress.Body.String(), source.Code, source.Body.String())
	}
	resume := httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(resume, request)
	if !strings.Contains(resume.Body.String(), `data-start="15"`) {
		t.Fatalf("bundled resume did not use chopped timeline: %q", resume.Body.String())
	}
}

func assertAutomaticSkipSubtitle(t *testing.T, handler http.Handler, token, body string) {
	t.Helper()
	subtitlePath := regexp.MustCompile(`<track[^>]+(?:data-subtitle-source|src)="(/subtitle/[^"]+)"`).FindStringSubmatch(body)
	if len(subtitlePath) != 2 {
		t.Fatalf("chopped player subtitle URL = %q", body)
	}
	subtitle := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, subtitlePath[1], nil)
	request.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(subtitle, request)
	if subtitle.Code != http.StatusOK || !strings.Contains(subtitle.Body.String(), "Inside intro") || !strings.Contains(subtitle.Body.String(), "00:00:12.000 --> 00:00:14.000") {
		t.Fatalf("direct player subtitle = %d %q", subtitle.Code, subtitle.Body.String())
	}
}
