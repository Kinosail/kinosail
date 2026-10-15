package server

import "github.com/MikeO7/kinosail/packages/mcpgateway"

var mcpReadRoutes = routeSet(
	"GET /api/v1", "GET /api/v1/openapi.json", "GET /api/v1/me", "GET /api/v1/library", "GET /api/v1/subtitle-library",
)

var mcpWriteRoutes = routeSet(
	"PUT /api/v1/me/language",
)

var mcpManageRoutes = routeSet(
	"POST /api/v1/subtitle-operations", "GET /api/v1/subtitle-operations/{operation}", "GET /api/v1/subtitle-operations/{operation}/result",
	"GET /api/v1/subtitle-library/{id}/draft", "POST /api/v1/subtitle-library/{id}/draft", "GET /api/v1/subtitle-library/{id}/inspect", "POST /api/v1/subtitle-library/{id}/preview", "POST /api/v1/subtitle-library/{id}/apply", "POST /api/v1/subtitle-library/{id}/audio",
	"GET /api/v1/agent-connections", "GET /api/v1/backups", "GET /api/v1/configuration",
	"POST /api/v1/subtitle-library/fetch-wanted", "POST /api/v1/subtitle-library/maintain", "POST /api/v1/subtitle-library/{id}/fetch", "POST /api/v1/subtitle-library/{id}/replacement", "POST /api/v1/subtitle-library/{id}/restore", "POST /api/v1/subtitle-providers/test",
	"GET /api/v1/diagnostics", "GET /api/v1/maintenance",
	"GET /api/v1/metrics", "GET /api/v1/remote-access", "GET /api/v1/settings", "GET /api/v1/updates",
	"DELETE /api/v1/agent-connections/{id}",
	"PUT /api/v1/items/{id}/metadata", "POST /api/v1/items/{id}/metadata/refresh",
	"POST /api/v1/metadata/bulk",
	"POST /api/v1/libraries", "DELETE /api/v1/libraries",
	"PUT /api/v1/settings/server", "PUT /api/v1/settings/navigation", "PUT /api/v1/settings/onboarding", "PUT /api/v1/settings/subtitles",
	"PUT /api/v1/settings/scans", "PUT /api/v1/settings/trusted-https", "PUT /api/v1/settings/updates", "DELETE /api/v1/settings/trusted-https",
	"POST /api/v1/tasks/{task}", "POST /api/v1/backups", "POST /api/v1/backups/verify",
	"POST /api/v1/updates", "POST /api/v1/updates/check",
)

var mcpBlockedRoutes = routeSet(
	"PUT /api/v1/configuration/{key}", "DELETE /api/v1/configuration/{key}",
	"GET /api/v1/activity",
	"POST /api/v1/subtitles/cleanup", "POST /api/v1/subtitles/cleanup/preview",
	"DELETE /api/v1/management-access", "DELETE /api/v1/management-access/devices", "GET /api/v1/management-access", "POST /api/v1/management-access", "POST /api/v1/management-access/devices",
	"GET /api/v1/subtitle-library/{id}/export",
	"DELETE /api/v1/api-keys/{id}", "DELETE /api/v1/devices/{id}", "DELETE /api/v1/passkeys/{id}",
	"DELETE /api/v1/me/mfa", "DELETE /api/v1/me/oidc", "DELETE /api/v1/me/saml", "DELETE /api/v1/profiles/{id}",
	"DELETE /api/v1/remote-access/kill", "DELETE /api/v1/session", "DELETE /api/v1/sessions",
	"GET /api/v1/activity/export", "GET /api/v1/agent-connections/certificate", "GET /api/v1/api-keys", "GET /api/v1/backup",
	"GET /api/v1/devices", "GET /api/v1/events", "GET /api/v1/passkeys",
	"GET /api/v1/me/oidc/link", "GET /api/v1/me/saml/link", "GET /api/v1/profiles",
	"GET /api/v1/session/oidc", "GET /api/v1/session/saml", "GET /api/v1/supporter", "GET /api/v1/supporter/certificates/{family}",
	"POST /api/v1/api-keys", "POST /api/v1/me/mfa/setup",
	"POST /api/v1/passkeys/login/begin", "POST /api/v1/passkeys/login/finish",
	"POST /api/v1/passkeys/register/begin", "POST /api/v1/passkeys/register/finish",
	"POST /api/v1/profiles",
	"POST /api/v1/remote-access/kill",
	"POST /api/v1/session", "POST /api/v1/setup", "POST /api/v1/supporter/activate",
	"PUT /api/v1/me/mfa", "PUT /api/v1/profiles/{id}", "PUT /api/v1/profiles/{id}/password", "PUT /api/v1/settings/mfa", "PUT /api/v1/settings/session-timeouts", "PUT /api/v1/settings/public-session-timeouts", "DELETE /api/v1/settings/public-session-timeouts",
)

type mcpRoutePolicy struct{}

func (mcpRoutePolicy) Allows(pattern string, access mcpgateway.AccessClass) bool {
	switch access {
	case mcpgateway.ReadAccess:
		return mcpReadRoutes[pattern]
	case mcpgateway.WriteAccess:
		return mcpWriteRoutes[pattern]
	case mcpgateway.ManageAccess:
		return mcpManageRoutes[pattern]
	default:
		return false
	}
}
