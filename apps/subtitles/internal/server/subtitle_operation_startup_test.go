package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// A live Server lifecycle schedules its initial automatic-maintenance pass.
// These two restart fixtures must reach their durable-write/child-start
// boundary after that existing work settles. Every busy response must leave
// the same public receipt reusable and start no analysis or file/history work.
// Deliberate busy/replay controls retain the ordinary activation helper.
func activateSubtitleOperationAfterStartup(t *testing.T, handler http.Handler, base, id, target, calls string) *httptest.ResponseRecorder {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		response := activateSubtitleOperation(t, handler, base+"/audio", `{"language":"en"}`, []string{id})
		if response.Code != http.StatusConflict {
			return response
		}
		assertSubtitleStartupBusyHasNoEffects(t, handler, id, target, calls)
		if !time.Now().Before(deadline) {
			t.Fatal("startup admission prerequisite remained busy within its two-second bound")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func assertSubtitleStartupBusyHasNoEffects(t *testing.T, handler http.Handler, id, target, calls string) {
	t.Helper()
	state := readSubtitleOperation(t, handler, id)
	if state.State != "prepared" || state.Status != 0 || state.Outcome != "" {
		t.Fatalf("startup admission conflict consumed the prepared receipt: %+v", state)
	}
	assertSubtitleOperationNoProcess(t, calls)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	if _, err := os.Stat(target + ".kinosail.bak"); !os.IsNotExist(err) {
		t.Fatalf("startup admission conflict created recovery data: %v", err)
	}
	_ = subtitleActionHistory(t, handler, nil, nil)
	t.Log("startup admission prerequisite: same receipt prepared, zero analysis starts, unchanged current, recovery absent, History empty")
}
