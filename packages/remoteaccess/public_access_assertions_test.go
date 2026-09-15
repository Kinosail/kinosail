package remoteaccess_test

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/remoteaccess"
)

func assertUntrustedPublicRequestsRejected(t *testing.T, client *http.Client, address string) {
	t.Helper()
	unknownName := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: "attacker.example", MinVersion: tls.VersionTLS13}}} //nolint:gosec // The handshake must fail before certificate verification.
	if rejected, err := doGet(t, unknownName, "https://"+address); err == nil {
		_ = rejected.Body.Close()
		t.Fatal("unknown public SNI completed a TLS handshake")
	}
	badHost, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://"+address, nil)
	if err != nil {
		t.Fatal(err)
	}
	badHost.Host = "attacker.example"
	if rejected, requestErr := client.Do(badHost); requestErr != nil {
		t.Fatal(requestErr)
	} else {
		_ = rejected.Body.Close()
		if rejected.StatusCode != http.StatusMisdirectedRequest {
			t.Fatalf("unknown public Host = %d", rejected.StatusCode)
		}
	}
	if plain, plainErr := doGet(t, http.DefaultClient, "http://"+address); plainErr == nil {
		_ = plain.Body.Close()
		if plain.StatusCode < http.StatusBadRequest {
			t.Fatalf("plain HTTP was accepted with status %d", plain.StatusCode)
		}
	}
}

func assertKillClosesPublicAccess(t *testing.T, manager *remoteaccess.Manager, client *http.Client, address string, activeStarted, activeCanceled <-chan struct{}) {
	t.Helper()
	activeDone := make(chan struct{})
	go func() {
		defer close(activeDone)
		request, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://"+address+"/hold", nil)
		request.Host = "family-media.duckdns.org"
		if held, err := client.Do(request); err == nil {
			_, _ = io.ReadAll(held.Body)
			_ = held.Body.Close()
		}
	}()
	select {
	case <-activeStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("active public request did not start")
	}
	if err := manager.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-activeCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("kill switch did not cancel active public request")
	}
	<-activeDone
	client.CloseIdleConnections()
	closed, err := doGet(t, client, "https://"+address)
	if closed != nil {
		_ = closed.Body.Close()
	}
	if err == nil || manager.Status().State != "killed" {
		t.Fatalf("kill switch left public HTTPS available: status=%#v err=%v", manager.Status(), err)
	}
}
