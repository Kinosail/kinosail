package jellyfincompat

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/playback"
)

func subtitleRequest(t *testing.T, index, query string) *http.Request {
	t.Helper()
	request := deliveryRequest(t, "stream", query)
	request.SetPathValue("index", index)
	return request
}

func TestDeliverySubtitleSelectsExternalAndTimelineModes(t *testing.T) {
	t.Parallel()
	state := newDeliveryState(t)
	module := state.module()
	response := httptest.NewRecorder()
	module.Subtitle(response, subtitleRequest(t, "0", ""))
	if state.subtitle != "film.vtt" || state.timeline != nil {
		t.Fatalf("external subtitle = %q timeline=%#v", state.subtitle, state.timeline)
	}
	state.subtitle = ""
	plan := state.plan
	plan.MarkerMode = "server"
	plan.Timeline = playback.Timeline{SourceDuration: 60, Duration: 50}
	state.session = deliveryTestSession{item: state.item.ID, profile: "viewer", revision: 2, expires: state.now.Add(1), plan: plan}
	module.Subtitle(httptest.NewRecorder(), subtitleRequest(t, "0", "playSessionId=play"))
	if state.subtitle != "film.vtt" || state.timeline == nil || state.timeline.Duration != 50 {
		t.Fatalf("mapped subtitle = %q timeline=%#v", state.subtitle, state.timeline)
	}
	for _, index := range []string{"bad", "-1", "1025", "1"} {
		missing := httptest.NewRecorder()
		module.Subtitle(missing, subtitleRequest(t, index, ""))
		if missing.Code != http.StatusNotFound {
			t.Fatalf("invalid index %q = %d", index, missing.Code)
		}
	}
	state.visible = false
	hidden := httptest.NewRecorder()
	module.Subtitle(hidden, subtitleRequest(t, "0", ""))
	if hidden.Code != http.StatusNotFound {
		t.Fatalf("hidden subtitle = %d", hidden.Code)
	}
}

func TestDeliverySubtitleHandlesEmbeddedPlayerAndSubtitlesFailures(t *testing.T) { //nolint:cyclop // Both product error contracts use one policy.
	t.Parallel()
	state := newDeliveryState(t)
	state.media.Facts.Subtitles = []playback.SubtitleFacts{{SourceIndex: 2, Text: true}}
	module := state.module()
	module.Subtitle(httptest.NewRecorder(), subtitleRequest(t, "2", ""))
	if state.embedded != 1 {
		t.Fatalf("direct embedded calls = %d", state.embedded)
	}
	plan := state.plan
	plan.MarkerMode = "server"
	plan.Timeline = playback.Timeline{SourceDuration: 10, Duration: 10}
	state.session = deliveryTestSession{item: state.item.ID, profile: "viewer", revision: 2, expires: state.now.Add(1), plan: plan}
	state.extractData = []byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nText\n")
	mapped := httptest.NewRecorder()
	module.Subtitle(mapped, subtitleRequest(t, "2", "playSessionId=play"))
	if mapped.Code != http.StatusOK || !strings.Contains(mapped.Body.String(), "WEBVTT") || mapped.Header().Get("Content-Type") != "text/vtt; charset=utf-8" {
		t.Fatalf("mapped embedded = %d %q", mapped.Code, mapped.Body.String())
	}
	file := filepath.Join(t.TempDir(), "subtitle.vtt")
	if err := os.WriteFile(file, state.extractData, 0o600); err != nil {
		t.Fatal(err)
	}
	state.extractPath, state.extractData = file, nil
	fromFile := httptest.NewRecorder()
	module.Subtitle(fromFile, subtitleRequest(t, "2", "playSessionId=play"))
	if !strings.Contains(fromFile.Body.String(), "Text") {
		t.Fatalf("file embedded = %d %q", fromFile.Code, fromFile.Body.String())
	}
	state.extractPath, state.extractErr = "", errors.New("extract")
	player := httptest.NewRecorder()
	module.Subtitle(player, subtitleRequest(t, "2", "playSessionId=play"))
	if player.Code != http.StatusNotFound || state.unavailable != 1 {
		t.Fatalf("Player embedded failure = %d unavailable=%d", player.Code, state.unavailable)
	}
	module.Policy.EmbeddedFailureStatus = http.StatusServiceUnavailable
	subtitles := httptest.NewRecorder()
	module.Subtitle(subtitles, subtitleRequest(t, "2", "playSessionId=play"))
	if subtitles.Code != http.StatusServiceUnavailable || subtitles.Body.String() != "embedded subtitles are unavailable\n" || state.unavailable != 2 {
		t.Fatalf("Subtitles embedded failure = %d %q unavailable=%d", subtitles.Code, subtitles.Body.String(), state.unavailable)
	}
	state.media.Facts.Subtitles = []playback.SubtitleFacts{{SourceIndex: 2, Text: false}}
	state.item.Subtitles = []string{"one"}
	missing := httptest.NewRecorder()
	module.Subtitle(missing, subtitleRequest(t, "2", ""))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("non-text embedded = %d", missing.Code)
	}
}
