package server_test

import (
	"net/http"
	"testing"
)

// Completion is a progress event, not an explicit departure from the page.
// The web player saves zero at ended; native progress can retain its end time.
func TestWatchedHTTPAllowsSameSessionReplayAfterCompletion(t *testing.T) {
	for _, adapter := range []string{"web", "API"} {
		for _, seconds := range []string{"0", "120"} {
			t.Run(adapter+"/completed-seconds-"+seconds, func(t *testing.T) {
				fixture := newWatchedSessionHTTP(t)
				fixture.progress(t, adapter, "replaying-page", "1", "119", fixture.success(adapter))
				fixture.complete(t, adapter, seconds)
				before := fixture.state(t)
				if !before.Watched || before.Session != "replaying-page" || before.Revision != 2 {
					t.Fatal("ordinary completion did not commit its ordered session")
				}
				fixture.progress(t, adapter, "replaying-page", "2", "7", fixture.rejected(adapter))
				fixture.unchanged(t, before)
				fixture.progress(t, adapter, "replaying-page", "1", "7", fixture.rejected(adapter))
				fixture.unchanged(t, before)
				fixture.restart(t)
				fixture.unchanged(t, before)
				fixture.progress(t, adapter, "replaying-page", "3", "7", fixture.success(adapter))
				if after := fixture.state(t); after.Watched || after.Seconds != 7 || after.Revision != 3 {
					t.Fatal("explicit replay in the completed session could not save progress")
				}
				fixture.mark(t, true)
				before = fixture.state(t)
				fixture.progress(t, adapter, "replaying-page", "4", "8", fixture.rejected(adapter))
				fixture.unchanged(t, before)
			})
		}
	}
}

func (fixture *watchedSessionHTTP) complete(t *testing.T, adapter, seconds string) {
	t.Helper()
	if adapter == "web" {
		fixture.request(t, http.MethodPost, "/progress/"+fixture.id, "application/x-www-form-urlencoded", "seconds="+seconds+"&watched=true&session=replaying-page&revision=2", http.StatusNoContent)
		return
	}
	fixture.request(t, http.MethodPut, "/api/v1/items/"+fixture.id+"/progress", "application/json", `{"seconds":`+seconds+`,"watched":true,"session":"replaying-page","revision":2}`, http.StatusOK)
}
