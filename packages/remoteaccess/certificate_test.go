package remoteaccess_test

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/remoteaccess"
)

func TestExpiredPublicCertificateFailsClosedWithoutExposingCredentials(t *testing.T) {
	t.Parallel()
	duck := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte("OK")) }))
	defer duck.Close()
	address := unusedAddress(t)
	token := strings.Repeat("z", 32)
	manager, err := remoteaccess.New(remoteaccess.Config{Enabled: true, PublicHTTPS: true, Domain: "family-media", Token: token, Listen: address, DataDir: t.TempDir()}, remoteaccess.Dependencies{Client: duck.Client(), UpdateURL: duck.URL, Certificate: testCertificateValidity(t, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))})
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = manager.Serve(t.Context(), http.NotFoundHandler()) }()
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: "family-media.duckdns.org", MinVersion: tls.VersionTLS13}}} //nolint:gosec // The server must reject its own expired test certificate.
	for range 100 {
		var response *http.Response
		response, err = doGet(t, client, "https://"+address)
		if response != nil {
			_ = response.Body.Close()
		}
		if manager.Status().State == "error" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err == nil || manager.Status().State != "error" || strings.Contains(manager.Status().Error, token) {
		t.Fatalf("expired certificate status = %#v, %v", manager.Status(), err)
	}
}
