package transcodepolicy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestCheckCoordinatorProjectsPendingAndStoredResults(t *testing.T) {
	lock := &sync.RWMutex{}
	stored := CheckResult{}
	recorded := CheckResult{}
	coordinator := NewCheckCoordinator(CheckCoordinatorDependencies{
		Lock: lock, Result: &stored, FFmpeg: "",
		Resolve:     func() (Settings, error) { return Settings{Codec: "h264", Accelerator: "none"}, nil },
		Pending:     func() Settings { return Settings{Codec: "hevc", Accelerator: "qsv"} },
		BackendName: func(string) string { return "Software" },
		RecordHardware: func(_ Settings, result CheckResult) {
			recorded = result
		},
	})

	if pending := coordinator.Current(); pending.Status != "not-run" || pending.Codec != "hevc" || pending.Backend != "Software" {
		t.Fatalf("pending check = %#v", pending)
	}
	result := coordinator.Run(context.Background())
	if recorded != result || stored != result || coordinator.Current() != result {
		t.Fatalf("stored check = %#v, recorded = %#v, current = %#v", stored, recorded, coordinator.Current())
	}
}

func TestCheckHandlerRunsAndRedirects(t *testing.T) {
	called := false
	handler := CheckHandler(func(context.Context) CheckResult {
		called = true
		return CheckResult{}
	})
	request := httptest.NewRequest(http.MethodPost, "/settings/transcoder/test", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !called || response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/settings#transcoder" {
		t.Fatalf("check handler = called:%v status:%d location:%q", called, response.Code, response.Header().Get("Location"))
	}
}
