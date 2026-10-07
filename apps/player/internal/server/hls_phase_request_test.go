package server

import (
	"net/http"
	"testing"
)

func (fixture phaseHTTPFixture) request(t *testing.T, item int, token, requestID string, session ...string) <-chan int {
	t.Helper()
	result := make(chan int, 1)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, fixture.host.URL+"/hls/"+fixture.ids[item]+"/p/"+token+"/index.m3u8", nil)
	if err != nil {
		t.Fatal("cannot create synthetic HLS request")
	}
	request.Header.Set("X-Request-ID", requestID)
	request.Header.Set("X-Playback-Session", "private-phase-session")
	if len(session) > 0 {
		request.Header.Set("X-Playback-Session", session[0])
	}
	go func() {
		response, failure := fixture.client.Do(request)
		if failure != nil {
			result <- 0
			return
		}
		_ = response.Body.Close()
		result <- response.StatusCode
	}()
	return result
}
