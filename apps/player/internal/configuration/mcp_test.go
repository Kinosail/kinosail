package configuration_test

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
)

func TestMCPConfigurationRequiresTheCompleteOAuthResource(t *testing.T) {
	_, err := configuration.Load(t.TempDir(), "", func(name string) (string, bool) {
		if name == "KINOSAIL_MCP_RESOURCE_URL" {
			return "https://media.example.com/mcp", true
		}
		return "", false
	})
	if err == nil || !strings.Contains(err.Error(), "all MCP OAuth settings") {
		t.Fatalf("partial MCP configuration error = %v", err)
	}
}

func TestMCPConfigurationRequiresSecureCanonicalURLs(t *testing.T) {
	values := map[string]string{
		"KINOSAIL_MCP_RESOURCE_URL":         "http://media.example.com/not-mcp",
		"KINOSAIL_MCP_AUTHORIZATION_SERVER": "https://identity.example.com",
		"KINOSAIL_MCP_INTROSPECTION_URL":    "https://identity.example.com/introspect",
		"KINOSAIL_MCP_CLIENT_ID":            "resource",
		"KINOSAIL_MCP_CLIENT_SECRET":        "secret",
	}
	_, err := configuration.Load(t.TempDir(), "", func(name string) (string, bool) { value, ok := values[name]; return value, ok })
	if err == nil || !strings.Contains(err.Error(), "MCP URLs must use HTTPS") {
		t.Fatalf("insecure MCP configuration error = %v", err)
	}
}
