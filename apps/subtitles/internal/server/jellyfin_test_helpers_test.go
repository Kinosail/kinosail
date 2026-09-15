package server_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func decodeJellyfin(t *testing.T, response *httptest.ResponseRecorder, value any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), value); err != nil {
		t.Fatalf("decode %d %q: %v", response.Code, response.Body.String(), err)
	}
}
