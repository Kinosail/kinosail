package server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAgentConnectionAPIAndWebAdaptersRejectUnknownRevocation(t *testing.T) { //nolint:cyclop // One fixture proves projection and fail-closed revocation through both adapters.
	profiles := newProfileStore("")
	profiles.profiles = []viewerProfile{{ID: "owner", Name: "Owner", Owner: true}}
	connections := newMCPConnections("https://kino.test:38128", t.TempDir(), profiles, nil)

	response := httptest.NewRecorder()
	apiAgentConnections(connections).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/agent-connections", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"recommended":"stdio"`) || !strings.Contains(response.Body.String(), `docker exec -i kinosail kinosail mcp-stdio`) || !strings.Contains(response.Body.String(), `"requiresCertificate":false`) || !strings.Contains(response.Body.String(), `"resource":"https://kino.test:38128/mcp"`) || !strings.Contains(response.Body.String(), `codex mcp add kinosail-http --url https://kino.test:38128/mcp`) {
		t.Fatalf("agent connections API = %d %q", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	showAgentConnections(connections, "").ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings/agent-connections", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "Recommended: host connection") || !strings.Contains(body, "No certificate or browser approval is required") || !strings.Contains(body, "docker exec -i kinosail kinosail mcp-stdio") || !strings.Contains(body, "HTTPS / OAuth alternative") || !strings.Contains(body, "codex mcp login kinosail-http") || strings.Index(body, "Recommended: host connection") > strings.Index(body, "HTTPS / OAuth alternative") || !strings.Contains(body, "Codex") {
		t.Fatalf("agent connections page = %d %q", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/v1/agent-connections/connection", nil)
	request.SetPathValue("id", "connection")
	apiRevokeAgentConnection(connections).ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || len(connections.Views()) != 0 {
		t.Fatalf("API revocation = %d %q views=%#v", response.Code, response.Body.String(), connections.Views())
	}

	response = httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/agent-connections/revoke", strings.NewReader("id=web"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	webRevokeAgentConnection(connections).ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || len(connections.Views()) != 0 {
		t.Fatalf("web revocation = %d %q views=%#v", response.Code, response.Body.String(), connections.Views())
	}
}

func TestAgentConnectionRevocationRejectsMalformedInputWithoutSideEffects(t *testing.T) {
	connections := newMCPConnections("https://kino.test:38128", t.TempDir(), newProfileStore(""), nil)
	for name, body := range map[string]string{"missing": "", "duplicate": "id=keep&id=keep", "unknown": "id=keep&extra=true", "oversized": "id=" + strings.Repeat("x", 129)} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/agent-connections/revoke", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			webRevokeAgentConnection(connections).ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || len(connections.Views()) != 0 {
				t.Fatalf("malformed revocation = %d views=%#v", response.Code, connections.Views())
			}
		})
	}
}

func TestAgentConnectionCertificateDownloadNeverExposesPrivateKey(t *testing.T) {
	directory := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Kinosail Local CA", Organization: []string{"Kinosail generated"}}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	certificate, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	private, _ := x509.MarshalPKCS8PrivateKey(key)
	var identity bytes.Buffer
	_ = pem.Encode(&identity, &pem.Block{Type: "CERTIFICATE", Bytes: certificate})
	_ = pem.Encode(&identity, &pem.Block{Type: "PRIVATE KEY", Bytes: private})
	if err := os.WriteFile(filepath.Join(directory, "tls-ca.pem"), identity.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	connections := newMCPConnections("https://kino.test:38128", directory, newProfileStore(""), nil)
	response := httptest.NewRecorder()
	apiAgentConnectionCertificate(connections).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/agent-connections/certificate", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "BEGIN CERTIFICATE") || strings.Contains(response.Body.String(), "PRIVATE KEY") || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("certificate download = %d cache=%q %q", response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
}
