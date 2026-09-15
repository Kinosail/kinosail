package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func assertSafariDefersMatroska(t *testing.T, handler http.Handler, id string) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil)
	request.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 Version/18.6 Mobile/15E148 Safari/604.1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if strings.Contains(response.Body.String(), `src="/media/`+id+`?playbackSession=`) || !strings.Contains(response.Body.String(), `data-direct="/media/`+id+`?playbackSession=`) {
		t.Fatalf("Safari player started Matroska directly: %q", response.Body.String())
	}
}
