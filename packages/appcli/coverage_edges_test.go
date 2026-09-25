package appcli

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/remoteaccess"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
	"github.com/MikeO7/kinosail/packages/updatecontrol"
)

func TestBuiltInCommandRemainingEdges(t *testing.T) {
	t.Parallel()
	unusedUpdate := updatecontrol.Command(func(string, io.Reader, io.Writer, string, string, string) error { return nil })
	for _, test := range []struct {
		name      string
		dataDir   string
		listen    string
		wantError bool
	}{
		{name: "healthcheck", listen: "invalid", wantError: true},
		{name: "tls-certificate", dataDir: t.TempDir()},
	} {
		handled, err := builtInCommand(test.name, nil, io.Discard, test.dataDir, test.listen, false, "", unusedUpdate, "dev")
		if !handled || (err != nil) != test.wantError {
			t.Errorf("%s = handled %v, error %v", test.name, handled, err)
		}
	}
}

func TestAccessManagerAndTrustedHTTPSInitialization(t *testing.T) {
	t.Parallel()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("OK")), Header: make(http.Header)}, nil
	})}
	internet, err := remoteaccess.New(remoteaccess.Config{Enabled: true, PublicHTTPS: true, Domain: "family", Token: strings.Repeat("t", 32), Listen: listener.Addr().String(), DataDir: t.TempDir()}, remoteaccess.Dependencies{Client: client, Certificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return nil, errors.New("unused") }})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 1)
	startAccessManagers(t.Context(), "https", &http.Server{Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}, internet, nil, func(handler http.Handler) http.Handler {
		started <- struct{}{}
		return handler
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("remote manager did not start")
	}
	time.Sleep(20 * time.Millisecond)

	config, err := trustedhttps.NewProviderConfig(trustedhttps.ProviderDuckDNS, "family", strings.Repeat("t", 32), "192.168.1.10", true)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := config.Encode()
	if err != nil {
		t.Fatal(err)
	}
	manager, err := TrustedHTTPS(testSettings{"tls.duckdns": raw, "paths.data": t.TempDir()})
	if err != nil || manager == nil {
		t.Fatalf("trusted HTTPS = %#v, %v", manager, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
