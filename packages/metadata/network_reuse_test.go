package metadata

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func countingProvider(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	connections := &atomic.Int32{}
	server := httptest.NewUnstartedServer(handler)
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.Start()
	t.Cleanup(server.Close)
	return server, connections
}

func TestProviderClientReusesConnectionsAfterErrorsAndConcurrency(t *testing.T) {
	server, connections := countingProvider(t, func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/missing") {
			http.Error(writer, strings.Repeat("not found ", 400), http.StatusNotFound)
			return
		}
		time.Sleep(20 * time.Millisecond)
		_, _ = writer.Write([]byte(`{"ok":true}`))
	})
	client := localIntegrationHTTPClient(5 * time.Second)
	for range 5 {
		var target map[string]bool
		if err := GetProviderJSON(t.Context(), client, server.URL+"/missing", "token", &target); err == nil {
			t.Fatal("missing provider document was accepted")
		}
	}
	if count := connections.Load(); count != 1 {
		t.Fatalf("sequential provider errors opened %d connections, want 1", count)
	}
	connections.Store(0)
	var workers sync.WaitGroup
	for range 9 {
		workers.Go(func() {
			for range 4 {
				var target map[string]bool
				if err := GetProviderJSON(t.Context(), client, server.URL+"/found", "token", &target); err != nil || !target["ok"] {
					t.Errorf("provider document = %#v, %v", target, err)
				}
			}
		})
	}
	workers.Wait()
	if count := connections.Load(); count > 9 {
		t.Fatalf("nine provider workers opened %d connections, want at most 9", count)
	}
}

func TestProviderClientPacesRequestsPerHost(t *testing.T) {
	server, _ := countingProvider(t, func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte(`{"ok":true}`)) })
	client := localIntegrationHTTPClient(5 * time.Second)
	started := time.Now()
	for range 30 {
		var target map[string]bool
		if err := GetProviderJSON(t.Context(), client, server.URL+"/paced", "token", &target); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(started); elapsed < 400*time.Millisecond {
		t.Fatalf("30 provider requests finished in %s, want pacing near 40 requests per second", elapsed)
	}
}

func TestProviderClientHonorsRetryAfterOnRateLimit(t *testing.T) {
	requests := atomic.Int32{}
	server, _ := countingProvider(t, func(writer http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			writer.Header().Set("Retry-After", "1")
			http.Error(writer, "slow down", http.StatusTooManyRequests)
			return
		}
		_, _ = writer.Write([]byte(`{"ok":true}`))
	})
	started := time.Now()
	var target map[string]bool
	if err := GetProviderJSON(t.Context(), localIntegrationHTTPClient(5*time.Second), server.URL+"/limited", "token", &target); err != nil || !target["ok"] {
		t.Fatalf("rate-limited provider document = %#v, %v", target, err)
	}
	if elapsed := time.Since(started); requests.Load() != 2 || elapsed < 900*time.Millisecond {
		t.Fatalf("requests = %d after %s, want a retry after the advertised delay", requests.Load(), elapsed)
	}
}
