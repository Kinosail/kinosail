package httpguard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func markedPublic(request *http.Request) bool { return request.Header.Get("X-Public") == "true" }

func publicRequest(t *testing.T, method, path, remote string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	request.Header.Set("X-Public", "true")
	request.RemoteAddr = remote
	return request
}

func TestGuardPublicRequestsPreservesClassificationAndEffectOrder(t *testing.T) {
	t.Parallel()
	var effects []string
	next := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		effects = append(effects, "next")
		writer.WriteHeader(http.StatusNoContent)
	})
	guarded := GuardPublicRequests(next, markedPublic,
		func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
			effects = append(effects, "error:"+message)
			http.Error(writer, message, status)
		},
		func(writer http.ResponseWriter, _ *http.Request) {
			effects = append(effects, "not-found")
			http.NotFound(writer, nil)
		},
		func(_ *http.Request, reason string, _ bool) { effects = append(effects, "audit:"+reason) },
	)

	local := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.env", nil)
	localResponse := httptest.NewRecorder()
	guarded.ServeHTTP(localResponse, local)
	if localResponse.Code != http.StatusNoContent || strings.Join(effects, ",") != "next" {
		t.Fatalf("local request = %d, %v", localResponse.Code, effects)
	}
	effects = nil
	remote := "203.0.113.8:4321"
	for attempt := 1; attempt <= 3; attempt++ {
		response := httptest.NewRecorder()
		guarded.ServeHTTP(response, publicRequest(t, http.MethodGet, "/.git/config", remote))
		if response.Code != http.StatusNotFound {
			t.Fatalf("scanner attempt %d = %d", attempt, response.Code)
		}
	}
	if strings.Join(effects, ",") != "not-found,not-found,audit:scanner probe,not-found" {
		t.Fatalf("scanner effects = %v", effects)
	}
	effects = nil
	blocked := httptest.NewRecorder()
	guarded.ServeHTTP(blocked, publicRequest(t, http.MethodGet, "/library", remote))
	if blocked.Code != http.StatusTooManyRequests || blocked.Header().Get("Retry-After") != "900" || strings.Join(effects, ",") != "error:public source is temporarily quarantined" {
		t.Fatalf("blocked request = %d, %v, %v", blocked.Code, blocked.Header(), effects)
	}
}

func TestGuardPublicRequestsAuditsRepeatedCredentialFailures(t *testing.T) {
	t.Parallel()
	var audits []string
	status := http.StatusUnauthorized
	next := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(status)
		writer.WriteHeader(http.StatusOK)
	})
	guarded := GuardPublicRequests(next, markedPublic,
		func(writer http.ResponseWriter, _ *http.Request, message string, code int) {
			http.Error(writer, message, code)
		},
		func(writer http.ResponseWriter, request *http.Request) { http.NotFound(writer, request) },
		func(_ *http.Request, reason string, _ bool) { audits = append(audits, reason) },
	)
	for attempt := 0; attempt < 10; attempt++ {
		response := httptest.NewRecorder()
		guarded.ServeHTTP(response, publicRequest(t, http.MethodPost, "/api/v1/session", "203.0.113.9:4321"))
	}
	if strings.Join(audits, ",") != "repeated credential failure" {
		t.Fatalf("credential audits = %v", audits)
	}
	status = http.StatusForbidden
	for attempt := 0; attempt < 10; attempt++ {
		guarded.ServeHTTP(httptest.NewRecorder(), publicRequest(t, http.MethodPost, "/login", "203.0.113.10:4321"))
	}
	if len(audits) != 2 {
		t.Fatalf("forbidden credential audits = %v", audits)
	}
	status = http.StatusOK
	guarded.ServeHTTP(httptest.NewRecorder(), publicRequest(t, http.MethodPost, "/login/mfa", "203.0.113.11:4321"))
	guarded.ServeHTTP(httptest.NewRecorder(), publicRequest(t, http.MethodGet, "/library", "203.0.113.12:4321"))
	if len(audits) != 2 {
		t.Fatalf("safe requests changed audits: %v", audits)
	}
	recorder := httptest.NewRecorder()
	capture := &statusWriter{ResponseWriter: recorder}
	if capture.Unwrap() != recorder {
		t.Fatal("status writer did not expose its response writer")
	}
	if _, err := capture.Write([]byte("ok")); err != nil || capture.status != http.StatusOK {
		t.Fatalf("implicit response status = %d, %v", capture.status, err)
	}
}

func TestLimitPublicRequestsRejectsBeforeHandlerEffects(t *testing.T) {
	t.Parallel()
	called := 0
	next := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		called++
		writer.WriteHeader(http.StatusNoContent)
	})
	limited := LimitPublicRequests(next, markedPublic, func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
		http.Error(writer, message, status)
	})
	local := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", strings.NewReader("body"))
	limited.ServeHTTP(httptest.NewRecorder(), local)
	if called != 1 {
		t.Fatal("local request was limited")
	}
	for name, request := range map[string]*http.Request{
		"GET body":      publicRequest(t, http.MethodGet, "/", "203.0.113.1:1"),
		"HEAD transfer": publicRequest(t, http.MethodHead, "/", "203.0.113.1:1"),
		"range":         publicRequest(t, http.MethodGet, "/", "203.0.113.1:1"),
	} {
		request.ContentLength = 0
		switch name {
		case "GET body":
			request.ContentLength = 1
		case "HEAD transfer":
			request.TransferEncoding = []string{"chunked"}
		case "range":
			request.Header.Set("Range", "bytes=1-0")
		}
		response := httptest.NewRecorder()
		limited.ServeHTTP(response, request)
		want := http.StatusBadRequest
		if name == "range" {
			want = http.StatusRequestedRangeNotSatisfiable
		}
		if response.Code != want || called != 1 {
			t.Fatalf("%s = %d, called=%d", name, response.Code, called)
		}
	}
	post := publicRequest(t, http.MethodPost, "/", "203.0.113.1:1")
	post.ContentLength = 1
	limited.ServeHTTP(httptest.NewRecorder(), post)
	if called != 2 {
		t.Fatal("public POST body was rejected")
	}
}

func TestLimitPublicRequestsAppliesCapacityAndReleasesIt(t *testing.T) {
	entered := make(chan struct{}, sourceConcurrentLimit)
	release := make(chan struct{})
	next := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		entered <- struct{}{}
		<-release
		writer.WriteHeader(http.StatusNoContent)
	})
	limited := LimitPublicRequests(next, markedPublic, func(writer http.ResponseWriter, _ *http.Request, message string, status int) {
		http.Error(writer, message, status)
	})
	var group sync.WaitGroup
	for range sourceConcurrentLimit {
		group.Add(1)
		go func() {
			defer group.Done()
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			request.Header.Set("X-Public", "true")
			request.RemoteAddr = "203.0.113.20:4321"
			limited.ServeHTTP(httptest.NewRecorder(), request)
		}()
	}
	for range sourceConcurrentLimit {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("request did not enter the handler")
		}
	}
	blocked := httptest.NewRecorder()
	limited.ServeHTTP(blocked, publicRequest(t, http.MethodGet, "/", "203.0.113.20:9999"))
	if blocked.Code != http.StatusServiceUnavailable || blocked.Header().Get("Retry-After") != "1" {
		t.Fatalf("capacity response = %d, %v", blocked.Code, blocked.Header())
	}
	close(release)
	group.Wait()
	response := httptest.NewRecorder()
	limited.ServeHTTP(response, publicRequest(t, http.MethodGet, "/", "203.0.113.20:9999"))
	if response.Code != http.StatusNoContent {
		t.Fatalf("released capacity = %d", response.Code)
	}
}
