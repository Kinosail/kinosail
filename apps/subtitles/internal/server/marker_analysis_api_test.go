package server_test

import (
	"net/http"
	"testing"
)

func TestMarkerAnalysisReportsDetectorVersion(t *testing.T) {
	t.Parallel()
	handler, token := apiServer(t)
	var status struct {
		DetectorVersion int `json:"detectorVersion"`
	}
	mustJSON(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/marker-analysis", nil), &status)
	if status.DetectorVersion <= 0 {
		t.Fatalf("detector version = %d", status.DetectorVersion)
	}
}
