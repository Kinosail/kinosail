package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type jellyfinBitrateFixture struct {
	New    func(*testing.T) (http.Handler, string)
	Enable func(*testing.T, http.Handler, string)
	Call   func(*testing.T, http.Handler, string, string, string, string) *httptest.ResponseRecorder
}

// JellyfinBitrate checks authenticated transfer sizes and bounded invalid-input responses.
func JellyfinBitrate(t *testing.T,
	newHandler func(*testing.T) (http.Handler, string),
	enable func(*testing.T, http.Handler, string),
	call func(*testing.T, http.Handler, string, string, string, string) *httptest.ResponseRecorder,
) {
	t.Helper()
	fixture := jellyfinBitrateFixture{New: newHandler, Enable: enable, Call: call}
	t.Run("StreamsBoundedAuthenticatedBytes", fixture.testStreamsBoundedAuthenticatedBytes)
	t.Run("RejectsInvalidInputWithoutLargeResponse", fixture.testRejectsInvalidInputWithoutLargeResponse)
}

func (fixture jellyfinBitrateFixture) testStreamsBoundedAuthenticatedBytes(t *testing.T) {
	t.Parallel()
	handler, token := fixture.New(t)
	fixture.Enable(t, handler, token)

	for _, test := range []struct {
		query string
		size  int
	}{{"", 102400}, {"?size=1", 1}, {"?size=10000000", 10000000}} {
		response := fixture.Call(t, handler, http.MethodGet, "/Playback/BitrateTest"+test.query, "", token)
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/octet-stream" || response.Body.Len() != test.size {
			t.Fatalf("bitrate test %q = %d type=%q bytes=%d", test.query, response.Code, response.Header().Get("Content-Type"), response.Body.Len())
		}
	}
	if response := fixture.Call(t, handler, http.MethodGet, "/Playback/BitrateTest?size=1", "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous bitrate test = %d %q", response.Code, response.Body.String())
	}
}

func (fixture jellyfinBitrateFixture) testRejectsInvalidInputWithoutLargeResponse(t *testing.T) {
	t.Parallel()
	handler, token := fixture.New(t)
	fixture.Enable(t, handler, token)

	for _, query := range []string{
		"?size=0", "?size=-1", "?size=nope", "?size=10000001", "?size=1&size=2", "?unknown=1", "?size=" + strings.Repeat("1", 32),
	} {
		response := fixture.Call(t, handler, http.MethodGet, "/Playback/BitrateTest"+query, "", token)
		if response.Code != http.StatusBadRequest || response.Body.Len() > 1024 {
			t.Fatalf("invalid bitrate test %q = %d bytes=%d", query, response.Code, response.Body.Len())
		}
	}
}
