package server

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/quickconnect"
)

func TestQuickConnectBoundsPendingRequests(t *testing.T) {
	t.Parallel()
	broker := newQuickConnect(time.Minute)
	for range 1024 {
		if _, _, err := broker.connections.Create(quickconnect.Request{}); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect", nil)
	response := httptest.NewRecorder()
	broker.start(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("pending Quick Connect = %d", response.Code)
	}
}

func TestQuickConnectRejectsAmbiguousFormsWithoutPendingState(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"duplicate device": "device=Living+Room&device=Bedroom",
		"unknown field":    "device=Living+Room&extra=true",
	} {
		t.Run(name, func(t *testing.T) {
			broker := newQuickConnect(time.Minute)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := httptest.NewRecorder()
			broker.start(response, request)
			if response.Code != http.StatusBadRequest || broker.starts.TrackedClients() != 0 {
				t.Fatalf("invalid Quick Connect form = %d, limiter clients = %d", response.Code, broker.starts.TrackedClients())
			}
			assertQuickConnectPendingCapacity(t, broker)
		})
	}
}

func TestQuickConnectRejectsInvalidTransportWithoutPendingState(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		path        string
		contentType string
		body        string
	}{
		"JSONP media type": {path: "/api/v1/quick-connect", contentType: "application/jsonp", body: `{"device":"TV"}`},
		"JSON query":       {path: "/api/v1/quick-connect?device=TV", contentType: "application/json", body: `{"device":"TV"}`},
		"duplicate JSON":   {path: "/api/v1/quick-connect", contentType: "application/json", body: `{"device":"TV","device":"Bedroom"}`},
		"oversized body":   {path: "/api/v1/quick-connect", contentType: "application/json", body: `{"device":"` + strings.Repeat("a", quickConnectRequestBodyMaximum) + `"}`},
	} {
		t.Run(name, func(t *testing.T) {
			broker := newQuickConnect(time.Minute)
			body := strings.NewReader(test.body)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, test.path, body)
			request.Header.Set("Content-Type", test.contentType)
			response := httptest.NewRecorder()
			broker.start(response, request)
			if response.Code != http.StatusBadRequest || broker.starts.TrackedClients() != 0 {
				t.Fatalf("invalid Quick Connect transport = %d, limiter clients = %d", response.Code, broker.starts.TrackedClients())
			}
			if name == "oversized body" && body.Size()-int64(body.Len()) > quickConnectRequestBodyMaximum+1 {
				t.Fatalf("oversized body read = %d bytes", body.Size()-int64(body.Len()))
			}
			assertQuickConnectPendingCapacity(t, broker)
		})
	}
}

func TestQuickConnectAcceptsUnnamedDeviceFromEmptyChunkedBody(t *testing.T) {
	t.Parallel()
	broker := newQuickConnect(time.Minute)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect", nil)
	request.Body = io.NopCloser(strings.NewReader(""))
	request.ContentLength = -1
	response := httptest.NewRecorder()
	broker.start(response, request)
	if response.Code != http.StatusCreated || broker.starts.TrackedClients() != 1 {
		t.Fatalf("empty chunked Quick Connect = %d %q, start limiter clients=%d", response.Code, response.Body.String(), broker.starts.TrackedClients())
	}
	for range 1023 {
		if _, _, err := broker.connections.Create(quickconnect.Request{}); err != nil {
			t.Fatalf("empty chunked request did not create one pending request: %v", err)
		}
	}
	if _, _, err := broker.connections.Create(quickconnect.Request{}); !errors.Is(err, quickconnect.ErrCapacity) {
		t.Fatalf("pending capacity after empty chunked request = %v", err)
	}
}

func TestQuickConnectThrottlesMalformedRequestsBeforeParsing(t *testing.T) {
	t.Parallel()
	broker := newQuickConnect(time.Minute)
	var response *httptest.ResponseRecorder
	for range 61 {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect", strings.NewReader("{"))
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = "203.0.113.45:1234"
		response = httptest.NewRecorder()
		broker.start(response, request)
	}
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" || broker.starts.TrackedClients() != 0 {
		t.Fatalf("malformed Quick Connect limit = %d headers=%v, start limiter clients=%d", response.Code, response.Header(), broker.starts.TrackedClients())
	}
	assertQuickConnectPendingCapacity(t, broker)
}

func assertQuickConnectPendingCapacity(t *testing.T, broker *quickConnectBroker) {
	t.Helper()
	for range 1024 {
		if _, _, err := broker.connections.Create(quickconnect.Request{}); err != nil {
			t.Fatalf("invalid request created pending state: %v", err)
		}
	}
}

func TestQuickConnectPollingIsIndependentlyRateLimited(t *testing.T) {
	t.Parallel()
	assertQuickConnectPollLimit(t, "secret=unknown", "application/x-www-form-urlencoded", "203.0.113.44:1234")
}

func TestQuickConnectPollingThrottlesMalformedInputBeforeParsing(t *testing.T) {
	t.Parallel()
	assertQuickConnectPollLimit(t, "{", "application/json", "203.0.113.46:1234")
}

func assertQuickConnectPollLimit(t *testing.T, body, contentType, remoteAddr string) {
	t.Helper()
	broker := newQuickConnect(time.Minute)
	handler := broker.poll(newProfileStore(t.TempDir()))
	var response *httptest.ResponseRecorder
	for range 121 {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/quick-connect/token", strings.NewReader(body))
		request.Header.Set("Content-Type", contentType)
		request.RemoteAddr = remoteAddr
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
	}
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" {
		t.Fatalf("poll limit = %d headers=%v", response.Code, response.Header())
	}
}
