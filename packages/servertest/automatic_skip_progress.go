package servertest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func (fixture AutomaticSkipFixture) ServerAutomaticSkipMapsPlaybackProgressBackToSourceTime(t *testing.T) {
	t.Helper()
	handler, apiToken, token, rawID, id, _ := fixture.server(t)
	playback := jellyfinPlaybackRequest(t, handler, token, id, "Swiftfin", "1.6")
	var result struct {
		PlaySessionID string `json:"PlaySessionId"`
	}
	decodeAutomaticSkipJellyfin(t, playback, &result)
	progress := AutomaticSkipJellyfinRequest(t, handler, token, http.MethodPost, "/Sessions/Playing/Progress", "Swiftfin", "1.6", fmt.Sprintf(`{"ItemId":%q,"PositionTicks":150000000,"PlaySessionId":%q}`, id, result.PlaySessionID))
	presentation := AutomaticSkipJellyfinRequest(t, handler, token, http.MethodGet, "/Items/"+id, "Swiftfin", "1.6", "")
	source := APICall(t, handler, apiToken, http.MethodGet, "/api/v1/items/"+rawID, nil)
	if progress.Code != http.StatusNoContent || !strings.Contains(presentation.Body.String(), `"PlaybackPositionTicks":150000000`) || !strings.Contains(source.Body.String(), `"seconds":25`) {
		t.Fatalf("mapped progress = %d %q, presentation = %d %q, source = %d %q", progress.Code, progress.Body.String(), presentation.Code, presentation.Body.String(), source.Code, source.Body.String())
	}
}

func (fixture AutomaticSkipFixture) VersionedPlaybackMapsProgressBackToSourceTime(t *testing.T) {
	t.Helper()
	handler, token, _, id, _, _ := fixture.server(t)
	setCompatiblePlayback(t, handler, token)
	playback := APICall(t, handler, token, http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	var result struct {
		ProgressToken string `json:"progressToken"`
	}
	MustJSON(t, playback, &result)
	saved := APICall(t, handler, token, http.MethodPut, "/api/v1/items/"+id+"/progress", map[string]any{"seconds": 15, "playbackToken": result.ProgressToken})
	item := APICall(t, handler, token, http.MethodGet, "/api/v1/items/"+id, nil)
	AssertAPIBody(t, saved, http.StatusOK, `"seconds":15`)
	AssertAPIBody(t, item, http.StatusOK, `"seconds":25`)
	invalid := APICall(t, handler, token, http.MethodPut, "/api/v1/items/"+id+"/progress", map[string]any{"seconds": 15, "playbackToken": "not-a-playback-token"})
	AssertAPIBody(t, invalid, http.StatusBadRequest, `"error":"playback token is invalid"`)
}

func setCompatiblePlayback(t *testing.T, handler http.Handler, token string) {
	t.Helper()
	response := APICall(t, handler, token, http.MethodPut, "/api/v1/settings/playback", map[string]any{"mode": "compatible", "autoplay": true, "subtitles": "on", "autoSkip": []string{"intro"}})
	AssertAPIBody(t, response, http.StatusOK, `"status":"saved"`)
}

func (fixture AutomaticSkipFixture) ServerAutomaticSkipKeepsSubtitleCuesOnTheShortenedTimeline(t *testing.T) {
	t.Helper()
	handler, _, token, _, id, _ := fixture.server(t)
	playback := jellyfinPlaybackRequest(t, handler, token, id, "Swiftfin", "1.6")
	match := regexp.MustCompile(`"DeliveryUrl":"([^"]+Subtitles[^"]+)"`).FindStringSubmatch(playback.Body.String())
	if len(match) != 2 {
		t.Fatalf("playback subtitle URL = %q", playback.Body.String())
	}
	subtitle := httptest.NewRecorder()
	path := match[1]
	if fixture.UnescapeSubtitleQuery {
		path = strings.ReplaceAll(path, `\u0026`, "&")
	}
	handler.ServeHTTP(subtitle, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	if subtitle.Code != http.StatusOK || strings.Contains(subtitle.Body.String(), "Inside intro") || !strings.Contains(subtitle.Body.String(), "00:00:12.000 --> 00:00:14.000") || !strings.Contains(subtitle.Body.String(), "After intro") {
		t.Fatalf("mapped subtitle = %d %q", subtitle.Code, subtitle.Body.String())
	}
}
