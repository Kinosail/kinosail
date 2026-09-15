package main

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail/packages/servertransport"
)

func TestHealthCheckCoversHTTPHTTPSAndCanonicalHost(t *testing.T) { //nolint:cyclop // Transport, host routing, response, and validation are one health boundary.
	t.Parallel()
	hostSeen := ""
	httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		hostSeen = request.Host
		writer.WriteHeader(http.StatusOK)
	}))
	defer httpServer.Close()
	if err := servertransport.CheckHealth(httpServer.Listener.Addr().String(), false, "https://family.duckdns.org:38127"); err != nil || hostSeen != "family.duckdns.org:38127" {
		t.Fatalf("HTTP health = host:%q error:%v", hostSeen, err)
	}

	tlsServer := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusOK) }))
	tlsServer.TLS = &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13}
	tlsServer.StartTLS()
	defer tlsServer.Close()
	if err := servertransport.CheckHealth(tlsServer.Listener.Addr().String(), true, ""); err != nil {
		t.Fatalf("HTTPS health = %v", err)
	}

	badStatus := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusServiceUnavailable) }))
	defer badStatus.Close()
	if err := servertransport.CheckHealth(badStatus.Listener.Addr().String(), false, ""); err == nil {
		t.Fatal("unhealthy response accepted")
	}
	if err := servertransport.CheckHealth("invalid", false, ""); err == nil {
		t.Fatal("invalid listen address accepted")
	}
	if err := servertransport.CheckHealth(httpServer.Listener.Addr().String(), false, "https:///"); err == nil {
		t.Fatal("invalid authentication URL accepted")
	}
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	if err := servertransport.CheckHealth(address, false, ""); err == nil {
		t.Fatal("unreachable health endpoint accepted")
	}
}

func TestConfiguredMCPServerUsesOnlyTheDataDirectory(t *testing.T) {
	t.Parallel()
	configured, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) { return "/data", key == "KINOSAIL_DATA_DIR" })
	if err != nil {
		t.Fatal(err)
	}
	if config := configuredMCPServerConfig(configured); config.DataDir != "/data" {
		t.Fatalf("MCP server config = %#v", config)
	}
}
