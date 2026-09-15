package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var routeLoginIP atomic.Uint32

func TestCredentialLoginSuccessResetsOnlyThatAccount(t *testing.T) {
	t.Parallel()
	auth := authentication{}
	for attempt := 0; attempt < 10; attempt++ {
		if !auth.allowCredentialLogin("192.0.2."+strconv.Itoa(attempt+1)+":1234", "Owner") {
			t.Fatal("account was throttled before its limit")
		}
	}
	if auth.allowCredentialLogin("192.0.2.20:1234", "Owner") {
		t.Fatal("account was not throttled at its limit")
	}
	auth.credentialLoginSucceeded("Owner")
	if !auth.allowCredentialLogin("192.0.2.21:1234", "Owner") {
		t.Fatal("successful login did not reset its account limit")
	}
}

func TestPublicRequestsHavePerSourceConcurrencyBoundWithoutChangingLAN(t *testing.T) {
	entered := make(chan struct{}, 16)
	release := make(chan struct{})
	handler := Remote(publicRequestLimits(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		entered <- struct{}{}
		<-release
		writer.WriteHeader(http.StatusNoContent)
	})))
	var group sync.WaitGroup
	for range 16 {
		group.Add(1)
		go func() {
			defer group.Done()
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			request.RemoteAddr = "203.0.113.8:4321"
			handler.ServeHTTP(httptest.NewRecorder(), request)
		}()
	}
	for range 16 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("public request did not enter bounded handler")
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.RemoteAddr = "203.0.113.8:9999"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "1" {
		t.Fatalf("seventeenth public request = %d headers=%v", response.Code, response.Header())
	}
	close(release)
	group.Wait()

	local := publicRequestLimits(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) }))
	for range 17 {
		response := httptest.NewRecorder()
		local.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
		if response.Code != http.StatusNoContent {
			t.Fatalf("LAN request = %d", response.Code)
		}
	}
}

func TestPublicGetAndHeadRejectRequestBodiesBeforeHandlerSideEffects(t *testing.T) {
	t.Parallel()
	called := false
	handler := Remote(publicRequestLimits(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })))
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request := httptest.NewRequestWithContext(t.Context(), method, "/", strings.NewReader("body"))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || called {
			t.Fatalf("%s with body = %d, called=%v", method, response.Code, called)
		}
	}
}
