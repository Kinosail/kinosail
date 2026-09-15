package commandtest

import (
	"net/http"
	"testing"
	"time"
)

// HTTPServerLimits records the application-specific transport settings.
type HTTPServerLimits struct {
	Address           string
	Read, Write, Idle time.Duration
	HeaderBytes       int
}

// HTTPServerPolicy checks construction, common policy, and application limits.
func HTTPServerPolicy(t *testing.T, build func(string, http.Handler) *http.Server, limits HTTPServerLimits) {
	t.Helper()
	called := false
	server := build(limits.Address, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	if called {
		t.Fatal("HTTP server construction invoked the handler")
	}
	assertHTTPServerLimits(t, server, limits)
	if server.ErrorLog == nil || server.TLSConfig == nil || server.MaxHeaderValueCount != 64 || server.HTTP2 == nil {
		t.Fatalf("shared HTTP policy is incomplete: %#v", server)
	}
}

func assertHTTPServerLimits(t *testing.T, server *http.Server, limits HTTPServerLimits) {
	t.Helper()
	if server.Addr != limits.Address || server.ReadHeaderTimeout != 5*time.Second || server.ReadTimeout != limits.Read || server.WriteTimeout != limits.Write || server.IdleTimeout != limits.Idle || server.MaxHeaderBytes != limits.HeaderBytes {
		t.Fatalf("application HTTP limits = %#v; want %#v", server, limits)
	}
}
