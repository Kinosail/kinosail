package server_test

import (
	"net/http"
	"testing"
)

func TestViewerCanChooseMultipleSubtitleTracks(t *testing.T) {
	t.Parallel()
	fixture := libraryAPIFixture
	fixture.NewHandler = func(media, data string, requireAuth bool) http.Handler {
		handler := libraryAPIFixture.NewHandler(media, data, requireAuth)
		result := requestJSON(t, handler, http.MethodPut, "/api/v1/settings/subtitles", `{"languages":["en","es"]}`)
		if result.Code != http.StatusOK {
			t.Fatalf("preferred languages = %d %s", result.Code, result.Body.String())
		}
		return handler
	}
	fixture.ViewerCanChooseMultipleSubtitleTracks(t)
}
