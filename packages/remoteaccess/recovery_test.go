package remoteaccess

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestListenValidationRejectsBeforeFilesystemOrNetworkSideEffects(t *testing.T) {
	t.Parallel()
	for _, address := range []string{"", "bad", ":", ":http", ":0", ":-1", ":+443", ":0443", ":65536", "127.0.0.1:99999999999", "remote.example:443", "[invalid]:443", strings.Repeat("a", 256) + ":443"} {
		t.Run(address, func(t *testing.T) {
			directory := t.TempDir()
			manager, err := New(Config{Enabled: true, PublicHTTPS: true, Domain: "family", Token: strings.Repeat("a", 32), Listen: address, DataDir: directory})
			if err == nil || manager != nil {
				t.Fatalf("accepted %q", address)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 0 {
				t.Fatalf("rejection created state: %v %v", entries, err)
			}
		})
	}
	for _, address := range []string{":443", ":8443", "0.0.0.0:8443", "127.0.0.1:65535", "[::]:8443", "[::1]:443", "localhost:8443"} {
		if !validListen(address) {
			t.Fatalf("valid listener rejected: %s", address)
		}
	}
}

func TestResetRequiresStopAndRestartsCurrentManager(t *testing.T) {
	t.Parallel()
	config := Config{Enabled: true, PublicHTTPS: true, Domain: "family", Token: strings.Repeat("a", 32), Listen: "127.0.0.1:8443", DataDir: t.TempDir()}
	dependency := Dependencies{Certificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return nil, nil }}
	manager, err := New(config, dependency)
	if err != nil {
		t.Fatal(err)
	}
	manager.client = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return okResponse(), nil })}
	if manager.ResetKill() == nil || manager.Status().State != "starting" {
		t.Fatal("reset accepted a live manager")
	}
	killAndResetForRecovery(t, manager)
	listens := 0
	manager.operations.listen = func(context.Context, string, string) (net.Listener, error) {
		listens++
		return &failureListener{acceptErr: net.ErrClosed}, nil
	}
	if err = manager.Serve(t.Context(), http.NotFoundHandler()); err != nil || listens != 1 || manager.Status().State != "ready" {
		t.Fatalf("same manager did not resume: error=%v listens=%d status=%#v", err, listens, manager.Status())
	}
	restarted, err := New(config, dependency)
	if err != nil || restarted.killed || restarted.Status().State != "starting" {
		t.Fatalf("new process cannot start: %#v %v", restarted, err)
	}
}

func killAndResetForRecovery(t *testing.T, manager *Manager) {
	t.Helper()
	if err := manager.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := manager.ResetKill(); err != nil {
		t.Fatal(err)
	}
}
