package mcpgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// OAuthConfig configures the protected resource and token introspection.
type OAuthConfig struct {
	ResourceURL, AuthorizationServer, IntrospectionURL string
	ClientID, ClientSecret                             string
}

// Configured reports whether all external OAuth values form a valid configuration.
func (config OAuthConfig) Configured() bool {
	resource, resourceErr := url.Parse(config.ResourceURL)
	authorizationServer, authorizationErr := url.Parse(config.AuthorizationServer)
	introspection, introspectionErr := url.Parse(config.IntrospectionURL)
	return resourceErr == nil && authorizationErr == nil && introspectionErr == nil && resource.Path == "/mcp" && trustedURL(resource) && trustedURL(authorizationServer) && trustedURL(introspection) && config.ClientID != "" && config.ClientSecret != ""
}

func (adapter *Gateway) verifyToken(ctx context.Context, token string, request *http.Request) (*mcpauth.TokenInfo, error) {
	form := url.Values{"token": {token}, "token_type_hint": {"access_token"}}
	check, err := http.NewRequestWithContext(ctx, http.MethodPost, adapter.config.IntrospectionURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	check.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	check.Header.Set("Accept", "application/json")
	check.SetBasicAuth(adapter.config.ClientID, adapter.config.ClientSecret)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(check)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OAuth token introspection failed with status %d", response.StatusCode)
	}
	var result struct {
		Active bool            `json:"active"`
		Scope  string          `json:"scope"`
		Exp    int64           `json:"exp"`
		Sub    string          `json:"sub"`
		Aud    json.RawMessage `json:"aud"`
	}
	if err := httpguard.DecodeJSON(response.Body, 1<<20, &result, false); err != nil {
		return nil, err
	}
	if !validMCPToken(result.Active, result.Exp, result.Sub, result.Scope, result.Aud, adapter.config.ResourceURL) {
		return nil, mcpauth.ErrInvalidToken
	}
	profile, found := adapter.principals.ByOIDC(adapter.config.AuthorizationServer, result.Sub)
	if !found || !adapter.principals.Allowed(profile, request, time.Now()) {
		return nil, mcpauth.ErrInvalidToken
	}
	adapter.principals.Attribute(request, profile)
	return &mcpauth.TokenInfo{Scopes: strings.Fields(result.Scope), Expiration: time.Unix(result.Exp, 0), UserID: profile.ID}, nil
}

func validMCPToken(active bool, expiration int64, subject, scope string, audience json.RawMessage, resource string) bool {
	if !active || expiration <= 0 || subject == "" || len(subject) > 256 || len(scope) > 4096 {
		return false
	}
	fields := strings.Fields(scope)
	if len(fields) > 64 {
		return false
	}
	for _, field := range fields {
		if len(field) > 128 {
			return false
		}
	}
	return mcpAudience(audience, resource)
}

func mcpAudience(raw json.RawMessage, resource string) bool {
	if len(raw) > 32<<10 || resource == "" || len(resource) > 2048 {
		return false
	}
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return len(one) <= 2048 && one == resource
	}
	var many []string
	if json.Unmarshal(raw, &many) != nil || len(many) == 0 || len(many) > 16 {
		return false
	}
	return validMCPAudiences(many) && contains(many, resource)
}

func validMCPAudiences(audiences []string) bool {
	for _, value := range audiences {
		if len(value) > 2048 {
			return false
		}
	}
	return true
}

func mcpMetadataURL(resource string) string {
	parsed, err := url.Parse(resource)
	if err != nil {
		return ""
	}
	parsed.Path = "/.well-known/oauth-protected-resource" + parsed.EscapedPath()
	parsed.RawPath, parsed.RawQuery, parsed.Fragment = "", "", ""
	return parsed.String()
}
