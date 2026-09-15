package commandtest

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

// HTTPFactory binds configuration loading to the app's real HTTP constructor.
func HTTPFactory[Snapshot, Internet, Trusted any](load func() (Snapshot, error), build func(context.Context, Snapshot, *Internet, *Trusted) *http.Server) func(context.Context) *http.Server {
	return func(ctx context.Context) *http.Server {
		configured, _ := load()
		return build(ctx, configured, nil, nil)
	}
}

func LogLevels(t *testing.T, configuredLogLevel func(string) slog.Level) {
	if configuredLogLevel("debug") != -4 || configuredLogLevel("info") != 0 || configuredLogLevel("warn") != 4 || configuredLogLevel("error") != 8 {
		t.Fatal("configured log levels do not match slog levels")
	}
}

func ProxyCapability(t *testing.T, secureProxyConfiguration func(string, string) bool) {
	t.Parallel()
	if secureProxyConfiguration("short", "") || secureProxyConfiguration("", "https://media.example.com") || !secureProxyConfiguration("", "") || !secureProxyConfiguration(strings.Repeat("t", 32), "https://media.example.com") {
		t.Fatal("proxy capability validation failed")
	}
}

func HTTPBounds(t *testing.T, newHTTPServer func(context.Context) *http.Server, listen string) { //nolint:cyclop // One assertion protects the complete HTTP transport limit set.
	t.Parallel()
	server := newHTTPServer(context.Background())
	if server.Addr != listen || server.ReadHeaderTimeout != 5*time.Second || server.ReadTimeout != 15*time.Second || server.IdleTimeout != 2*time.Minute || server.MaxHeaderBytes != 64<<10 || server.MaxHeaderValueCount != 64 || server.WriteTimeout != 0 || server.ErrorLog == nil || server.HTTP2 == nil || server.HTTP2.MaxConcurrentStreams != 32 || server.HTTP2.MaxDecoderHeaderTableSize != 4096 || server.HTTP2.MaxEncoderHeaderTableSize != 4096 || server.HTTP2.MaxReadFrameSize != 16<<10 || server.HTTP2.MaxReceiveBufferPerConnection != 1<<20 || server.HTTP2.MaxReceiveBufferPerStream != 256<<10 || server.HTTP2.SendPingTimeout != 30*time.Second || server.HTTP2.PingTimeout != 10*time.Second || server.HTTP2.WriteByteTimeout != 30*time.Second {
		t.Fatalf("HTTP limits = %#v", server)
	}
}

func HTTPLogPrivacy(t *testing.T, newHTTPServer func(context.Context) *http.Server) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	server := newHTTPServer(context.Background())
	server.ErrorLog.Print("http: TLS handshake error from 192.168.1.86:63174: remote error: tls: unknown certificate")
	logged := output.String()
	if !strings.Contains(logged, `"msg":"HTTP transport error"`) || !strings.Contains(logged, `"detail":"http: TLS handshake error from [redacted]: remote error: tls: unknown certificate"`) || strings.Contains(logged, "192.168.1.86") || strings.Contains(logged, "63174") {
		t.Fatalf("transport log exposed client address: %s", logged)
	}
}
