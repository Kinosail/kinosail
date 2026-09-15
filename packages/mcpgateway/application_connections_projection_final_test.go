package mcpgateway

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

func TestApplicationProjectionOwnsPlayerInstructionsAndDocument(t *testing.T) {
	principals := &testPrincipals{values: map[string]Principal{}}
	connections := testConnections("https://kino.test:38127", principals, &memoryState{})
	var mutex sync.RWMutex
	profiles := []identitycore.Profile{{ID: "owner", Name: "Owner", Owner: true}, {ID: "viewer", Name: "Viewer"}}
	state := ProfileState{Mutex: &mutex, Profiles: &profiles}
	projection := connections.ApplicationProjection(state, t.TempDir())
	assertBuiltInApplicationProjection(t, projection)
	assertApplicationProjectionDocument(t, projection)
	assertApplicationProjectionJSON(t, projection)
	connections.external, connections.issuer, connections.resource = true, "https://identity.test", "https://resource.test/mcp"
	assertExternalApplicationProjection(t, connections.ApplicationProjection(state, t.TempDir()))
	if owners := connectionOwners(ProfileState{}); owners != nil {
		t.Fatalf("invalid profile state produced owners: %+v", owners)
	}
}

func assertBuiltInApplicationProjection(t *testing.T, projection ConnectionProjection) {
	t.Helper()
	if projection.Mode != "built-in" || projection.Issuer != "https://kino.test:38127" || projection.Resource != "https://kino.test:38127/mcp" || projection.CertificateAvailable {
		t.Fatalf("built-in projection = %+v", projection)
	}
	if !strings.Contains(projection.StdioCommand, "docker exec -i kinosail kinosail mcp-stdio") || !strings.Contains(projection.StdioSSHCommand, "ssh -T SERVER") || !reflect.DeepEqual(projection.HTTPCommands, []string{"codex mcp add kinosail-http --url https://kino.test:38127/mcp", "codex mcp login kinosail-http"}) {
		t.Fatalf("connection commands = %+v", projection)
	}
	if !reflect.DeepEqual(projection.Owners, []ConnectionOwner{{ID: "owner", Name: "Owner"}}) || len(projection.Connections) != 0 {
		t.Fatalf("safe projections = owners %+v, connections %+v", projection.Owners, projection.Connections)
	}
}

func assertApplicationProjectionDocument(t *testing.T, projection ConnectionProjection) {
	t.Helper()
	expected := map[string]any{
		"mode": "built-in", "issuer": "https://kino.test:38127", "resource": "https://kino.test:38127/mcp", "recommended": "stdio",
		"stdio": map[string]any{
			"command": projection.StdioCommand, "sshCommand": projection.StdioSSHCommand,
			"owners": projection.Owners, "requiresCertificate": false,
		},
		"https": map[string]any{
			"commands": projection.HTTPCommands, "requiresCertificate": true, "certificateAvailable": false,
		},
		"connections": []ConnectionView{},
	}
	if document := projection.APIDocument(); !reflect.DeepEqual(document, expected) {
		t.Fatalf("API document = %#v", document)
	}
}

func assertApplicationProjectionJSON(t *testing.T, projection ConnectionProjection) {
	t.Helper()
	encoded, err := json.Marshal(projection.APIDocument())
	if err != nil {
		t.Fatal(err)
	}
	exact := `{"connections":[],"https":{"certificateAvailable":false,"commands":["codex mcp add kinosail-http --url https://kino.test:38127/mcp","codex mcp login kinosail-http"],"requiresCertificate":true},"issuer":"https://kino.test:38127","mode":"built-in","recommended":"stdio","resource":"https://kino.test:38127/mcp","stdio":{"command":"codex mcp add kinosail -- docker exec -i kinosail kinosail mcp-stdio","owners":[{"id":"owner","name":"Owner"}],"requiresCertificate":false,"sshCommand":"codex mcp add kinosail -- ssh -T SERVER docker exec -i kinosail kinosail mcp-stdio"}}`
	if string(encoded) != exact {
		t.Fatalf("API bytes = %q", encoded)
	}
}

func assertExternalApplicationProjection(t *testing.T, projection ConnectionProjection) {
	t.Helper()
	if projection.Mode != "external" || projection.Issuer != "https://identity.test" || projection.Resource != "https://resource.test/mcp" {
		t.Fatalf("external projection = %+v", projection)
	}
}

func TestLocalCACertificateRejectsUntrustedAndOversizedFiles(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "tls-ca.pem")
	if certificate := LocalCACertificate(directory); certificate != nil {
		t.Fatalf("missing file produced certificate: %q", certificate)
	}
	for name, contents := range map[string][]byte{
		"oversized":   bytes.Repeat([]byte("x"), 64<<10+1),
		"not PEM":     []byte("not a certificate"),
		"wrong block": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("private")}),
		"corrupt DER": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("broken")}),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			if certificate := LocalCACertificate(directory); certificate != nil {
				t.Fatalf("untrusted file produced certificate: %q", certificate)
			}
		})
	}
	for name, template := range map[string]x509.Certificate{
		"not CA":        {Subject: pkix.Name{Organization: []string{"Kinosail generated"}}},
		"no owner":      {IsCA: true},
		"many owners":   {IsCA: true, Subject: pkix.Name{Organization: []string{"Kinosail generated", "Other"}}},
		"foreign owner": {IsCA: true, Subject: pkix.Name{Organization: []string{"Other"}}},
	} {
		t.Run(name, func(t *testing.T) {
			contents := createProjectionCertificate(t, template)
			if err := os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			if certificate := LocalCACertificate(directory); certificate != nil {
				t.Fatalf("untrusted certificate was returned: %q", certificate)
			}
		})
	}
}

func TestLocalCACertificateReturnsOnlyCanonicalPublicCertificate(t *testing.T) {
	directory := t.TempDir()
	contents := createProjectionCertificate(t, x509.Certificate{IsCA: true, Subject: pkix.Name{Organization: []string{"Kinosail generated"}}})
	contents = append(contents, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("private")})...)
	if err := os.WriteFile(filepath.Join(directory, "tls-ca.pem"), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	certificate := LocalCACertificate(directory)
	if !bytes.Contains(certificate, []byte("BEGIN CERTIFICATE")) || bytes.Contains(certificate, []byte("PRIVATE KEY")) {
		t.Fatalf("public certificate = %q", certificate)
	}
	var mutex sync.RWMutex
	profiles := []identitycore.Profile(nil)
	connections := testConnections("https://kino.test:38127", &testPrincipals{values: map[string]Principal{}}, &memoryState{})
	if projection := connections.ApplicationProjection(ProfileState{Mutex: &mutex, Profiles: &profiles}, directory); !projection.CertificateAvailable {
		t.Fatal("valid local CA was not advertised")
	}
}

func createProjectionCertificate(t *testing.T, template x509.Certificate) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template.SerialNumber = big.NewInt(1)
	template.NotBefore, template.NotAfter = time.Now().Add(-time.Minute), time.Now().Add(time.Hour)
	template.BasicConstraintsValid = true
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
