package mcpgateway

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
)

// ConnectionOwner is one safe Owner choice for a host MCP connection.
type ConnectionOwner struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ConnectionProjection is the complete application-facing connection state.
type ConnectionProjection struct {
	Mode, Issuer, Resource, StdioCommand, StdioSSHCommand string
	HTTPCommands                                          []string
	Owners                                                []ConnectionOwner
	Connections                                           []ConnectionView
	CertificateAvailable                                  bool
}

// ApplicationProjection returns Player's canonical agent connection instructions.
func (connections *Connections) ApplicationProjection(state ProfileState, dataDir string) ConnectionProjection {
	mode := "built-in"
	if connections.External() {
		mode = "external"
	}
	return ConnectionProjection{
		Mode: mode, Issuer: connections.Issuer(), Resource: connections.Resource(),
		StdioCommand:    "codex mcp add kinosail -- docker exec -i kinosail kinosail mcp-stdio",
		StdioSSHCommand: "codex mcp add kinosail -- ssh -T SERVER docker exec -i kinosail kinosail mcp-stdio",
		HTTPCommands:    []string{"codex mcp add kinosail-http --url " + connections.Resource(), "codex mcp login kinosail-http"},
		Owners:          connectionOwners(state), Connections: connections.Views(), CertificateAvailable: LocalCACertificate(dataDir) != nil,
	}
}

// APIDocument returns the stable JSON shape served by application adapters.
func (projection ConnectionProjection) APIDocument() map[string]any {
	return map[string]any{
		"mode": projection.Mode, "issuer": projection.Issuer, "resource": projection.Resource, "recommended": "stdio",
		"stdio":       map[string]any{"command": projection.StdioCommand, "sshCommand": projection.StdioSSHCommand, "owners": projection.Owners, "requiresCertificate": false},
		"https":       map[string]any{"commands": projection.HTTPCommands, "requiresCertificate": true, "certificateAvailable": projection.CertificateAvailable},
		"connections": projection.Connections,
	}
}

func connectionOwners(state ProfileState) []ConnectionOwner {
	if state.Mutex == nil || state.Profiles == nil {
		return nil
	}
	state.Mutex.RLock()
	defer state.Mutex.RUnlock()
	owners := make([]ConnectionOwner, 0)
	for _, profile := range *state.Profiles {
		if profile.Owner {
			owners = append(owners, ConnectionOwner{ID: profile.ID, Name: profile.Name})
		}
	}
	return owners
}

// LocalCACertificate returns only the validated public local CA certificate.
func LocalCACertificate(dataDir string) []byte {
	contents, err := os.ReadFile(filepath.Join(dataDir, "tls-ca.pem")) //nolint:gosec // Fixed installation-owned filename.
	if err != nil || len(contents) > 64<<10 {
		return nil
	}
	block, _ := pem.Decode(contents)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil || !certificate.IsCA || len(certificate.Subject.Organization) != 1 || certificate.Subject.Organization[0] != "Kinosail generated" {
		return nil
	}
	var public bytes.Buffer
	_ = pem.Encode(&public, &pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw})
	return public.Bytes()
}
