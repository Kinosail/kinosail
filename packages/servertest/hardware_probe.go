package servertest

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func AwaitHardwareProbe(t *testing.T, handler http.Handler, token string) {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		response := APICall(t, handler, token, http.MethodGet, "/api/v1/hardware", nil)
		if response.Code == http.StatusOK && !strings.Contains(response.Body.String(), `"probing":true`) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("hardware probe did not finish: %d %s", response.Code, response.Body.String())
		}
	}
}
