package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Race-instrumented template initialization competes with package tests on hosted runners.
const testServerStartTimeout = 90 * time.Second

func TestCommandLineServesCompatibleHTTPSByDefault(t *testing.T) { //nolint:cyclop,funlen,gocognit // One process-level scenario verifies the complete externally visible TLS policy.
	address := unusedAddress(t)
	dataDir := t.TempDir()
	environment := append(os.Environ(), "KINOSAIL_CLI_HELPER=1", "KINOSAIL_DATA_DIR="+dataDir, "KINOSAIL_LISTEN="+address, `KINOSAIL_TLS_HOSTS=["media.home","192.0.2.10"]`)
	command := startTestServer(t, environment)
	defer stopTestServer(command)

	insecure := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13}}} //nolint:gosec // The test explicitly verifies the generated untrusted certificate path.
	response, err := waitForResponse(t, insecure, "https://"+address+"/healthz")
	if err != nil {
		t.Fatalf("HTTPS did not become ready: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || response.TLS.Version != tls.VersionTLS13 {
		t.Fatalf("HTTPS = %d TLS %x", response.StatusCode, response.TLS.Version)
	}
	assertHealthcheck(t, environment)
	secureSuites := map[uint16]bool{tls.TLS_AES_128_GCM_SHA256: true, tls.TLS_AES_256_GCM_SHA384: true, tls.TLS_CHACHA20_POLY1305_SHA256: true}
	if !secureSuites[response.TLS.CipherSuite] {
		t.Fatalf("cipher suite = %x", response.TLS.CipherSuite)
	}
	if len(response.TLS.PeerCertificates) != 2 {
		t.Fatalf("certificate chain length = %d", len(response.TLS.PeerCertificates))
	}
	certificate, authority := response.TLS.PeerCertificates[0], response.TLS.PeerCertificates[1]
	if certificate.CheckSignatureFrom(authority) != nil || authority.CheckSignatureFrom(authority) != nil || !authority.IsCA || certificate.NotAfter.Sub(certificate.NotBefore) > 398*24*time.Hour {
		t.Fatal("default certificate is not a short-lived leaf signed by the local CA")
	}
	trustedResponse, trustErr := testGet(t, http.DefaultClient, "https://"+address+"/healthz")
	if trustedResponse != nil {
		_ = trustedResponse.Body.Close()
	}
	if trustErr == nil {
		t.Fatal("default trust unexpectedly accepted the generated certificate")
	}
	tls12 := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256}}}} //nolint:gosec // The test explicitly verifies the generated untrusted certificate path.
	tls12Response, tls12Err := testGet(t, tls12, "https://"+address+"/healthz")
	if tls12Err != nil {
		t.Fatalf("TLS 1.2 compatibility connection failed: %v", tls12Err)
	}
	_ = tls12Response.Body.Close()
	if tls12Response.TLS.Version != tls.VersionTLS12 || tls12Response.TLS.CipherSuite != tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256 {
		t.Fatalf("TLS 1.2 = version %x suite %x", tls12Response.TLS.Version, tls12Response.TLS.CipherSuite)
	}
	tls11 := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MaxVersion: tls.VersionTLS11}}} //nolint:gosec // This deliberately probes rejection of obsolete TLS.
	tls11Response, tls11Err := testGet(t, tls11, "https://"+address+"/healthz")
	if tls11Response != nil {
		_ = tls11Response.Body.Close()
	}
	if tls11Err == nil {
		t.Fatal("TLS 1.1 was accepted")
	}
	cbc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA}}}} //nolint:gosec // This deliberately probes rejection of a non-AEAD suite.
	cbcResponse, cbcErr := testGet(t, cbc, "https://"+address+"/healthz")
	if cbcResponse != nil {
		_ = cbcResponse.Body.Close()
	}
	if cbcErr == nil {
		t.Fatal("TLS 1.2 CBC cipher was accepted")
	}
	plainResponse, plainErr := testGet(t, http.DefaultClient, "http://"+address+"/healthz")
	if plainResponse != nil {
		_ = plainResponse.Body.Close()
	}
	if plainErr == nil && plainResponse.StatusCode < http.StatusBadRequest {
		t.Fatalf("plain HTTP = %d", plainResponse.StatusCode)
	}
	for _, name := range []string{"tls.pem", "tls-ca.pem"} {
		info, statErr := os.Stat(filepath.Join(dataDir, name))
		if statErr != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %v, %v", name, info, statErr)
		}
	}
	pool := x509.NewCertPool()
	pool.AddCert(authority)
	if _, err = certificate.Verify(x509.VerifyOptions{Roots: pool, DNSName: "localhost"}); err != nil {
		t.Fatalf("generated certificate cannot be explicitly trusted for localhost: %v", err)
	}
	for _, host := range []string{"media.home", "192.0.2.10"} {
		if err = certificate.VerifyHostname(host); err != nil {
			t.Fatalf("generated certificate does not match %s: %v", host, err)
		}
	}
}

func TestCommandLineCanDisableTLS(t *testing.T) {
	address := unusedAddress(t)
	dataDir := t.TempDir()
	environment := append(os.Environ(), "KINOSAIL_CLI_HELPER=1", "KINOSAIL_DATA_DIR="+dataDir, "KINOSAIL_LISTEN="+address, "KINOSAIL_TLS_ENABLED=false")
	command := startTestServer(t, environment)
	defer stopTestServer(command)
	response, err := waitForResponse(t, http.DefaultClient, "http://"+address+"/healthz")
	if err != nil {
		t.Fatalf("HTTP did not become ready: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("HTTP = %d", response.StatusCode)
	}
	assertHealthcheck(t, environment)
	insecure := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // Tests only.
	tlsResponse, tlsErr := testGet(t, insecure, "https://"+address+"/healthz")
	if tlsResponse != nil {
		_ = tlsResponse.Body.Close()
	}
	if tlsErr == nil {
		t.Fatal("HTTPS was accepted when TLS was disabled")
	}
	if _, err = os.Stat(filepath.Join(dataDir, "tls.pem")); !os.IsNotExist(err) {
		t.Fatalf("disabled TLS certificate = %v", err)
	}
}

func unusedAddress(t *testing.T) string {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	return address
}

func startTestServer(t *testing.T, environment []string) *exec.Cmd {
	t.Helper()
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=TestCommandLineHelperProcess", "--") //nolint:gosec // The executable is the current Go test binary.
	// TLS readiness must not depend on the runner's GPU or encoder inventory.
	ffmpeg := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(ffmpeg, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ffmpeg, 0o700); err != nil { //nolint:gosec // The owner-only test fixture must be executable.
		t.Fatal(err)
	}
	environment = append(environment, "KINOSAIL_FFMPEG="+ffmpeg, "KINOSAIL_TLS_TEST=1")
	command.Env = environment
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("server output: %s", output.String())
		}
	})
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	return command
}

func stopTestServer(command *exec.Cmd) {
	_ = command.Process.Signal(os.Interrupt)
	_ = command.Wait()
}

func waitForResponse(t *testing.T, client *http.Client, url string) (*http.Response, error) {
	t.Helper()
	var response *http.Response
	var err error
	for deadline := time.Now().Add(testServerStartTimeout); time.Now().Before(deadline); {
		response, err = testGet(t, client, url)
		if err == nil {
			return response, nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return response, err
}

func testGet(t *testing.T, client *http.Client, url string) (*http.Response, error) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return client.Do(request)
}

func assertHealthcheck(t *testing.T, environment []string) {
	t.Helper()
	healthcheck := exec.CommandContext(t.Context(), os.Args[0], "-test.run=TestCommandLineHelperProcess", "--", "healthcheck") //nolint:gosec // The executable is the current Go test binary.
	healthcheck.Env = environment
	if output, err := healthcheck.CombinedOutput(); err != nil {
		t.Fatalf("healthcheck = %v, %s", err, output)
	}
}
