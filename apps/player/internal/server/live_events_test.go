package server_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestLiveEventAPIStreamsThroughTheServerMiddleware(t *testing.T) {
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	cookie := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	invalid := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/events", nil)
	invalid.Header.Set("Last-Event-ID", "invalid")
	invalid.AddCookie(cookie)
	invalidResponse := httptest.NewRecorder()
	handler.ServeHTTP(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid event cursor = %d %q", invalidResponse.Code, invalidResponse.Body.String())
	}

	web := httptest.NewServer(handler)
	t.Cleanup(web.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, web.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(cookie)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	line, err := bufio.NewReader(response.Body).ReadString('\n')
	if err != nil || response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/event-stream" || response.Header.Get("Cache-Control") != "no-store" || !strings.HasPrefix(line, "retry: ") {
		t.Fatalf("event stream = %d %q %q, line %q, error %v", response.StatusCode, response.Header.Get("Content-Type"), response.Header.Get("Cache-Control"), line, err)
	}
}
