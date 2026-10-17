package server_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type heldMediaResponse struct {
	header  http.Header
	status  int
	started chan struct{}
	resume  chan struct{}
	done    chan struct{}
	once    sync.Once
}

func (response *heldMediaResponse) Header() http.Header { return response.header }

func (response *heldMediaResponse) WriteHeader(status int) { response.status = status }

func (response *heldMediaResponse) Write(data []byte) (int, error) {
	response.once.Do(func() { close(response.started) })
	<-response.resume
	return len(data), nil
}

func TestMediaShareDeviceKeepsOverlappingRangeRequestsAndRefusesSecondDevice(t *testing.T) {
	handler, owner := apiServer(t)
	var items struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	mustJSON(t, apiCall(t, handler, owner, http.MethodGet, "/api/v1/library", nil), &items)
	created := apiCall(t, handler, owner, http.MethodPost, "/api/v1/media-shares", map[string]any{"itemIds": []string{items.Items[0].ID}, "expiresInSeconds": 3600, "maxDevices": 1, "rightsAcknowledged": true})
	var share struct{ ClaimToken string }
	mustJSON(t, created, &share)
	claimed := apiCall(t, handler, "", http.MethodPost, "/api/v1/media-shares/claim", map[string]any{"token": share.ClaimToken, "device": "Safari"})
	if claimed.Code != http.StatusNoContent || len(claimed.Result().Cookies()) != 1 {
		t.Fatalf("claim = %d %q", claimed.Code, claimed.Body.String())
	}
	device := claimed.Result().Cookies()[0]
	ranged := func(byteRange string) *http.Request {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/share/media/"+items.Items[0].ID, nil)
		request.Header.Set("Range", byteRange)
		request.AddCookie(device)
		return request
	}
	held := &heldMediaResponse{header: make(http.Header), started: make(chan struct{}), resume: make(chan struct{}), done: make(chan struct{})}
	first := ranged("bytes=0-")
	go func() {
		defer close(held.done)
		handler.ServeHTTP(held, first)
	}()
	select {
	case <-held.started:
	case <-held.done:
		t.Fatalf("first range finished before streaming: %d", held.status)
	case <-time.After(5 * time.Second):
		t.Fatal("first range did not start streaming")
	}
	overlapping := httptest.NewRecorder()
	handler.ServeHTTP(overlapping, ranged("bytes=1-"))
	if held.status != http.StatusPartialContent || overlapping.Code != http.StatusPartialContent || overlapping.Body.String() != "ideo" {
		t.Fatalf("overlapping ranges = %d and %d %q", held.status, overlapping.Code, overlapping.Body.String())
	}
	second := apiCall(t, handler, "", http.MethodPost, "/api/v1/media-shares/claim", map[string]any{"token": share.ClaimToken, "device": "Second browser"})
	if second.Code != http.StatusTooManyRequests || len(second.Result().Cookies()) != 0 {
		t.Fatalf("second device claim = %d cookies=%d", second.Code, len(second.Result().Cookies()))
	}
	close(held.resume)
	<-held.done
	again := httptest.NewRecorder()
	handler.ServeHTTP(again, ranged("bytes=2-"))
	if again.Code != http.StatusPartialContent || again.Body.String() != "deo" {
		t.Fatalf("range after refused second device = %d %q", again.Code, again.Body.String())
	}
	if retry := apiCall(t, handler, "", http.MethodPost, "/api/v1/media-shares/claim", map[string]any{"token": share.ClaimToken, "device": "Second browser"}); retry.Code != http.StatusTooManyRequests {
		t.Fatalf("refused claim granted a device slot: %d", retry.Code)
	}
}
