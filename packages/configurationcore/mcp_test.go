package configurationcore

import (
	"strings"
	"testing"
)

func TestValidateMCPAcceptsDisabledAndCompleteConfigurations(t *testing.T) {
	t.Parallel()
	for name, configured := range map[string]testSnapshot{
		"disabled": {},
		"https": mcpSnapshot(
			"https://media.example.com/mcp",
			"https://identity.example.com/authorize",
			"https://identity.example.com/introspect",
		),
		"localhost": mcpSnapshot(
			"http://localhost/mcp",
			"http://127.0.0.1/authorize",
			"http://[::1]/introspect",
		),
	} {
		configured := configured
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateMCP(configured); err != nil {
				t.Fatalf("ValidateMCP() error = %v", err)
			}
		})
	}
}

func TestValidateMCPRejectsIncompleteAndUntrustedConfigurations(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		configured testSnapshot
		message    string
	}{
		"incomplete": {
			testSnapshot{mcpKeys[0]: "https://media.example.com/mcp"},
			"all MCP OAuth settings",
		},
		"malformed resource": {
			mcpSnapshot("https://%", "https://identity.example.com", "https://identity.example.com/introspect"),
			"MCP URLs must use HTTPS",
		},
		"wrong resource path": {
			mcpSnapshot("https://media.example.com/not-mcp", "https://identity.example.com", "https://identity.example.com/introspect"),
			"MCP URLs must use HTTPS",
		},
		"public HTTP": {
			mcpSnapshot("http://media.example.com/mcp", "https://identity.example.com", "https://identity.example.com/introspect"),
			"MCP URLs must use HTTPS",
		},
		"missing host": {
			mcpSnapshot("https:///mcp", "https://identity.example.com", "https://identity.example.com/introspect"),
			"MCP URLs must use HTTPS",
		},
		"user information": {
			mcpSnapshot("https://user@media.example.com/mcp", "https://identity.example.com", "https://identity.example.com/introspect"),
			"MCP URLs must use HTTPS",
		},
		"query": {
			mcpSnapshot("https://media.example.com/mcp?q=1", "https://identity.example.com", "https://identity.example.com/introspect"),
			"MCP URLs must use HTTPS",
		},
		"fragment": {
			mcpSnapshot("https://media.example.com/mcp#fragment", "https://identity.example.com", "https://identity.example.com/introspect"),
			"MCP URLs must use HTTPS",
		},
		"untrusted authorization server": {
			mcpSnapshot("https://media.example.com/mcp", "ftp://identity.example.com", "https://identity.example.com/introspect"),
			"MCP URLs must use HTTPS",
		},
		"untrusted introspection URL": {
			mcpSnapshot("https://media.example.com/mcp", "https://identity.example.com", "http://identity.example.com/introspect"),
			"MCP URLs must use HTTPS",
		},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateMCP(test.configured); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("ValidateMCP() error = %v, want %q", err, test.message)
			}
		})
	}
}

func mcpSnapshot(resource, authorization, introspection string) testSnapshot {
	return testSnapshot{
		mcpKeys[0]: resource,
		mcpKeys[1]: authorization,
		mcpKeys[2]: introspection,
		mcpKeys[3]: "resource",
		mcpKeys[4]: "secret",
	}
}
