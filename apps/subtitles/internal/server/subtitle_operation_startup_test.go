package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

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
