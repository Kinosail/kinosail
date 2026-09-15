package server_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestConcurrentSupporterActivationKeepsBothFamilies(t *testing.T) {
	signer := newSupporterSigner(t)
	signer.fixtures = map[string]supporterFixture{
		"CONCURRENT_PATRON_KEY": {tier: "admiral"},
		"CONCURRENT_LIVING_KEY": {tier: "commodore", sustaining: true},
	}
	upstream := httptest.NewServer(http.HandlerFunc(signer.handler))
	defer upstream.Close()
	handler, token := supporterServer(t, t.TempDir(), signer, upstream)
	responses := make(chan *httptest.ResponseRecorder, 2)
	var wait sync.WaitGroup
	for _, key := range []string{"CONCURRENT_PATRON_KEY", "CONCURRENT_LIVING_KEY"} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			responses <- apiCall(t, handler, token, http.MethodPost, "/api/v1/supporter/activate", map[string]any{"key": key})
		}()
	}
	wait.Wait()
	close(responses)
	for response := range responses {
		if response.Code != http.StatusOK {
			t.Fatalf("concurrent activation = %d %q", response.Code, response.Body.String())
		}
	}
	status := apiCall(t, handler, token, http.MethodGet, "/api/v1/supporter", nil)
	assertAPIBody(t, status, http.StatusOK, `"livingStandard":{"family":"living-standard"`, `"tier":"commodore"`, `"patronOrder":{"family":"patron-order"`, `"tier":"admiral"`)
}
