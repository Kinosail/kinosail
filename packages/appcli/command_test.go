package appcli

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/backup"
	"github.com/MikeO7/kinosail/packages/remoteaccess"
	"github.com/MikeO7/kinosail/packages/servertransport"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
	"github.com/MikeO7/kinosail/packages/updatecontrol"
)

type testSettings map[string]string

func (settings testSettings) Bool(key string) bool     { return settings[key] == "true" }
func (settings testSettings) String(key string) string { return settings[key] }
func (settings testSettings) Strings(string) []string  { return []string{"localhost"} }

func TestBoundCommandDispatchesAppAndSharedOperations(t *testing.T) {
	t.Parallel()
	called := ""
	command := Bind(
		backup.Command(func(args []string, _ io.Reader, _ io.Writer, _, _ string) (bool, error) {
			return args[0] == "backup", nil
		}),
		updatecontrol.Command(func(name string, _ io.Reader, _ io.Writer, _, _, _ string) error { called = name; return nil }),
		"v1.2.3",
	)
	for _, test := range []struct {
		name string
		args []string
		call string
	}{
		{"backup", []string{"backup"}, ""},
		{"artifact", []string{"update-artifact"}, "update-artifact"},
		{"plan", []string{"update-plan"}, "update-plan"},
		{"report", []string{"update-report"}, "update-report"},
	} {
		called = ""
		handled, err := command(test.args, strings.NewReader(""), io.Discard, "/data", "key", "", false, "")
		if !handled || err != nil || called != test.call {
			t.Errorf("%s = handled %v, call %q, error %v", test.name, handled, called, err)
		}
	}
	var output bytes.Buffer
	if handled, err := command([]string{"version"}, nil, &output, "", "", "", false, ""); !handled || err != nil || output.String() != "v1.2.3\n" {
		t.Fatalf("version = %v, %v, %q", handled, err, output.String())
	}
}

func TestBoundCommandRejectsUnknownAndUnsafeArguments(t *testing.T) {
	t.Parallel()
	backupCalls := 0
	command := Bind(
		backup.Command(func([]string, io.Reader, io.Writer, string, string) (bool, error) { backupCalls++; return false, nil }),
		updatecontrol.Command(func(string, io.Reader, io.Writer, string, string, string) error { return errors.New("unused") }),
		"dev",
	)
	for _, args := range [][]string{{"unknown"}, {"version", "extra"}, {strings.Repeat("x", 4097)}, {"a", "b", "c", "d"}} {
		if handled, err := command(args, nil, io.Discard, "", "", "", false, ""); !handled || err == nil {
			t.Errorf("unsafe arguments accepted: %#v", args)
		}
	}
	if handled, err := command(nil, nil, io.Discard, "", "", "", false, ""); handled || err != nil {
		t.Fatalf("server mode = %v, %v", handled, err)
	}
	if backupCalls != 2 {
		t.Fatalf("backup adapter calls = %d", backupCalls)
	}
	if handled, err := Bind(nil, nil, "dev")(nil, nil, io.Discard, "", "", "", false, ""); !handled || err == nil {
		t.Fatalf("missing adapters = %v, %v", handled, err)
	}
}

func TestBuiltInTransportCommandsPropagateErrors(t *testing.T) {
	t.Parallel()
	update := updatecontrol.Command(func(string, io.Reader, io.Writer, string, string, string) error { return nil })
	if handled, err := builtInCommand("healthcheck", nil, io.Discard, "", "invalid", false, "", update, "dev"); !handled || err == nil {
		t.Fatalf("invalid healthcheck = %v, %v", handled, err)
	}
	var certificate bytes.Buffer
	if handled, err := builtInCommand("tls-certificate", nil, &certificate, t.TempDir(), "", false, "", update, "dev"); !handled || err != nil || certificate.Len() == 0 {
		t.Fatalf("trust anchor = %v, %v, %d bytes", handled, err, certificate.Len())
	}
}

func TestLogLevelsFollowPlayerPolicy(t *testing.T) {
	t.Parallel()
	if LogLevel("debug") != -4 || LogLevel("info") != 0 || LogLevel("warn") != 4 || LogLevel("error") != 8 {
		t.Fatal("configured log levels do not match slog levels")
	}
}

func TestRunUsesSharedLifecycleAndToleratesOptionalManagerFailures(t *testing.T) {
	t.Parallel()
	settings := testSettings{"remote.mode": "https", "tls.enabled": "true", "paths.data": "/data"}
	remote, err := remoteaccess.New(remoteaccess.Config{})
	if err != nil {
		t.Fatal(err)
	}
	trusted, err := trustedhttps.New(trustedhttps.Config{}, "")
	if err != nil {
		t.Fatal(err)
	}
	remoteHandled := make(chan struct{}, 1)
	server := &http.Server{Addr: "127.0.0.1:0", Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), ReadHeaderTimeout: time.Second}
	exit := run(
		settings,
		nil,
		func(context.Context, *remoteaccess.Manager, *trustedhttps.Manager) *http.Server { return server },
		func(handler http.Handler) http.Handler { remoteHandled <- struct{}{}; return handler },
		func(Settings) (*remoteaccess.Manager, error) { return remote, nil },
		func(Settings) (*trustedhttps.Manager, error) { return trusted, nil },
		func(got *http.Server, config servertransport.TLSConfig) error {
			if got != server || !config.Enabled || config.DataDir != "/data" || config.Certificates != trusted {
				t.Fatalf("server configuration = %#v, %#v", got, config)
			}
			return nil
		},
	)
	if exit != 0 {
		t.Fatalf("exit = %d", exit)
	}
	select {
	case <-remoteHandled:
	case <-time.After(time.Second):
		t.Fatal("remote handler was not installed")
	}
}

func TestRunFailurePaths(t *testing.T) {
	t.Parallel()
	settings := testSettings{"remote.mode": "off"}
	managerFailure := errors.New("manager failed")
	build := func(_ context.Context, internet *remoteaccess.Manager, trusted *trustedhttps.Manager) *http.Server {
		if internet != nil || trusted != nil {
			t.Fatal("failed optional managers must be discarded")
		}
		return &http.Server{ReadHeaderTimeout: time.Second}
	}
	if exit := run(settings, nil, build, func(handler http.Handler) http.Handler { return handler },
		func(Settings) (*remoteaccess.Manager, error) { return nil, managerFailure },
		func(Settings) (*trustedhttps.Manager, error) { return nil, managerFailure },
		func(*http.Server, servertransport.TLSConfig) error { return errors.New("serve failed") }); exit != 1 {
		t.Fatalf("serve failure exit = %d", exit)
	}
	if exit := run(settings, nil, build, func(handler http.Handler) http.Handler { return handler },
		func(Settings) (*remoteaccess.Manager, error) { return nil, nil },
		func(Settings) (*trustedhttps.Manager, error) { return nil, nil },
		func(*http.Server, servertransport.TLSConfig) error { return http.ErrServerClosed }); exit != 0 {
		t.Fatalf("closed server exit = %d", exit)
	}
	if exit := run(settings, nil, func(context.Context, *remoteaccess.Manager, *trustedhttps.Manager) *http.Server { return nil }, func(handler http.Handler) http.Handler { return handler },
		func(Settings) (*remoteaccess.Manager, error) { return nil, nil },
		func(Settings) (*trustedhttps.Manager, error) { return nil, nil },
		func(*http.Server, servertransport.TLSConfig) error { return nil }); exit != 1 {
		t.Fatalf("missing server exit = %d", exit)
	}
	if exit := Run(nil, nil, nil, nil); exit != 1 {
		t.Fatalf("missing dependencies exit = %d", exit)
	}
}

func TestAccessConfigurationUsesPlayerPolicy(t *testing.T) { //nolint:cyclop // One table verifies the complete shared access configuration.
	t.Parallel()
	if remote, err := RemoteAccess(testSettings{"remote.mode": "off"}); err != nil || remote == nil {
		t.Fatalf("disabled remote access = %#v, %v", remote, err)
	}
	if remote, err := RemoteAccess(testSettings{"remote.mode": "https"}); err == nil || remote != nil {
		t.Fatalf("unsafe remote access = %#v, %v", remote, err)
	}
	if trusted, err := TrustedHTTPS(testSettings{}); err != nil || trusted != nil {
		t.Fatalf("disabled trusted HTTPS = %#v, %v", trusted, err)
	}
	if trusted, err := TrustedHTTPS(testSettings{"tls.duckdns": "invalid"}); err == nil || trusted != nil {
		t.Fatalf("invalid trusted HTTPS = %#v, %v", trusted, err)
	}
	trustedJSON := `{"provider":"duckdns","domain":"family-media","token":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","address":"192.168.1.2","termsAccepted":true}`
	if trusted, err := TrustedHTTPS(testSettings{"tls.duckdns": trustedJSON, "paths.data": t.TempDir()}); err != nil || trusted == nil {
		t.Fatalf("trusted HTTPS = %#v, %v", trusted, err)
	}
}

func TestRemoteAndShutdownFailuresCompleteLifecycle(t *testing.T) { //nolint:paralleltest // The test observes asynchronous lifecycle completion.
	duck := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("OK"))
	}))
	defer duck.Close()
	occupied, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	remote, err := remoteaccess.New(
		remoteaccess.Config{Enabled: true, PublicHTTPS: true, Domain: "family-media", Token: strings.Repeat("a", 32), Listen: occupied.Addr().String(), DataDir: t.TempDir()},
		remoteaccess.Dependencies{Client: duck.Client(), UpdateURL: duck.URL, Certificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return nil, errors.New("unused") }},
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	startAccessManagers(ctx, "https", &http.Server{Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}, remote, nil, func(handler http.Handler) http.Handler { return handler })
	for range 1000 {
		if remote.Status().Error != "" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if remote.Status().Error == "" {
		t.Fatal("remote listener failure did not complete")
	}
	time.Sleep(10 * time.Millisecond)
	cancel()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	wrapped := &closeFailureListener{Listener: listener, accepting: make(chan struct{}, 1)}
	server := &http.Server{Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(wrapped) }()
	<-wrapped.accepting
	shutdownCtx, stop := context.WithCancel(t.Context())
	shutdownDone := make(chan struct{})
	go func() { shutdownOnCancel(shutdownCtx, server); close(shutdownDone) }()
	stop()
	<-shutdownDone
	<-serveDone
}

type closeFailureListener struct {
	net.Listener
	accepting chan struct{}
}

func (listener *closeFailureListener) Accept() (net.Conn, error) {
	select {
	case listener.accepting <- struct{}{}:
	default:
	}
	return listener.Listener.Accept()
}

func (listener *closeFailureListener) Close() error {
	_ = listener.Listener.Close()
	return errors.New("close failed")
}
