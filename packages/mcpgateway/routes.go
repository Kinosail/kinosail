package mcpgateway

// Patterns returns every HTTP route owned by the shared MCP gateway.
func Patterns() []string {
	return []string{
		"GET /.well-known/oauth-protected-resource/mcp",
		"GET /mcp", "DELETE /mcp", "POST /mcp",
		"GET /.well-known/oauth-authorization-server",
		"POST /oauth/register", "GET /oauth/authorize", "POST /oauth/authorize",
		"POST /oauth/token", "POST /oauth/revoke",
	}
}
