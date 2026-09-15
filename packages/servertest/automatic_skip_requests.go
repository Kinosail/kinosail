package servertest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func jellyfinPlaybackRequest(t *testing.T, handler http.Handler, token, id, client, version string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"DeviceProfile":{"DirectPlayProfiles":[{"Type":"Video","Container":"mp4","VideoCodec":"h264","AudioCodec":"aac"}],"TranscodingProfiles":[{"Type":"Video","Container":"mp4","VideoCodec":"h264","AudioCodec":"aac","Protocol":"hls"}]}}`
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/Items/"+id+"/PlaybackInfo", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf(`MediaBrowser Client=%q, Device="Test", DeviceId="test", Version=%q, Token=%q`, client, version, token))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func AutomaticSkipJellyfinRequest(t *testing.T, handler http.Handler, token, method, path, client, version, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf(`MediaBrowser Client=%q, Device="Test", DeviceId="test", Version=%q, Token=%q`, client, version, token))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeAutomaticSkipJellyfin(t *testing.T, response *httptest.ResponseRecorder, value any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), value); err != nil {
		t.Fatalf("decode %d %q: %v", response.Code, response.Body.String(), err)
	}
}

func automaticSkipLibrary(t *testing.T, handler http.Handler, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Items?IncludeItemTypes=Episode", strings.NewReader(""))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", `MediaBrowser Client="Swiftfin", Device="iPhone", DeviceId="test", Version="1.5", Token="`+token+`"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
