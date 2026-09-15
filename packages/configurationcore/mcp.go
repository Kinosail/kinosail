package configurationcore

import (
	"errors"
	"net/url"
)

var mcpKeys = [...]string{
	"integrations.mcp.resource_url",
	"integrations.mcp.authorization_server",
	"integrations.mcp.introspection_url",
	"integrations.mcp.client_id",
	"integrations.mcp.client_secret",
}

var errMCPURL = errors.New("MCP URLs must use HTTPS (or localhost HTTP), and the resource URL must end at /mcp")

// StringSnapshot exposes resolved string configuration values.
type StringSnapshot interface {
	String(string) string
}

// ValidateMCP validates a complete OAuth resource configuration when MCP is enabled.
func ValidateMCP[S StringSnapshot](configured S) error {
	values := make([]string, len(mcpKeys))
	configuredValues := 0
	for index, key := range mcpKeys {
		values[index] = configured.String(key)
		if values[index] != "" {
			configuredValues++
		}
	}
	if configuredValues == 0 {
		return nil
	}
	if configuredValues != len(mcpKeys) {
		return errors.New("all MCP OAuth settings are required when MCP is enabled")
	}
	endpoints := make([]*url.URL, 3)
	for index, raw := range values[:3] {
		endpoint, err := url.Parse(raw)
		if err != nil || !trustedIntegrationURL(endpoint) {
			return errMCPURL
		}
		endpoints[index] = endpoint
	}
	if endpoints[0].Path != "/mcp" {
		return errMCPURL
	}
	return nil
}

func trustedIntegrationURL(endpoint *url.URL) bool {
	if endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return false
	}
	host := endpoint.Hostname()
	return endpoint.Scheme == "https" || endpoint.Scheme == "http" && (host == "localhost" || host == "127.0.0.1" || host == "::1")
}
