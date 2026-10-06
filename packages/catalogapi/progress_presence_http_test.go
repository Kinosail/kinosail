package catalogapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestProgressRequiresExplicitSecondsBeforeCallingStore(t *testing.T) {
	for _, body := range []string{
		"{}", "null", `{"seconds":null}`, `{"watched":true}`,
		`{"seconds":"3"}`, `{"seconds":1,"unknown":true}`,
		`{"seconds":1,"seconds":2}`, `{"seconds":1,"Seconds":2}`,
		"{", "[]", `{"seconds":1}{"seconds":2}`,
		strings.Repeat(" ", 1024*1024) + `{"seconds":0}`,
	} {
		t.Run(body[:min(len(body), 40)], func(t *testing.T) {
			handlers, _, progress, _ := mediaHandlersForTest(mediaFixture())
			progress.state.Seconds = 3
			response := callMedia(t, registeredMediaHandler(handlers), http.MethodPut, "/api/v1/items/movie/progress", body)
			if response.Code != http.StatusBadRequest || progress.progressSet != 0 || progress.state.Seconds != 3 {
				t.Fatalf("rejected progress status=%d storeCalls=%d savedSeconds=%v", response.Code, progress.progressSet, progress.state.Seconds)
			}
		})
	}
	handlers, _, progress, _ := mediaHandlersForTest(mediaFixture())
	progress.state.Seconds = 3
	response := callMedia(t, registeredMediaHandler(handlers), http.MethodPut, "/api/v1/items/movie/progress", `{"seconds":0}`)
	if response.Code != http.StatusOK || progress.progressSet != 1 || progress.state.Seconds != 0 {
		t.Fatal("explicit zero must still reset playback progress")
	}
}
