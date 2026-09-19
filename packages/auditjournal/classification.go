package auditjournal

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

func Classify(request *http.Request, overrides map[string]string) (string, string) { //nolint:cyclop,gocognit // Mutation names are intentionally centralized and exhaustive.
	if !auditMethod(request.Method) && request.URL.Path != "/login/oidc/callback" {
		return "", ""
	}
	path := request.URL.Path
	if playbackMutation(path) {
		return "", ""
	}
	if path == "/api/v1/session" {
		if request.Method == http.MethodDelete {
			return "session.ended", "security"
		}
		return "session.created", "security"
	}
	if path == "/Users/AuthenticateByName" || path == "/Users/AuthenticateWithQuickConnect" {
		return "session.created", "security"
	}
	if path == "/QuickConnect/Authorize" {
		return "quick-connect.approved", "security"
	}
	if action := staticAuditAction(request, overrides); action != "" {
		return action, "administration"
	}
	if action := dynamicAuditAction(request); action != "" {
		return action, map[bool]string{true: "administration", false: "activity"}[apiAdministrativeChange(request) || strings.HasPrefix(path, "/settings/") || strings.HasPrefix(path, "/metadata/") || strings.HasPrefix(path, "/markers/") || strings.HasPrefix(path, "/scim/v2/")]
	}
	if strings.HasPrefix(path, "/api/v1/configuration/") {
		return map[bool]string{true: "configuration.reset", false: "configuration.updated"}[request.Method == http.MethodDelete], "administration"
	}
	if apiAdministrativeChange(request) || strings.HasPrefix(path, "/settings/") || strings.HasPrefix(path, "/metadata/") || strings.HasPrefix(path, "/markers/") {
		return "administration." + strings.ToLower(request.Method), "administration"
	}
	return "activity." + strings.ToLower(request.Method), "activity"
}

func auditMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func staticAuditAction(request *http.Request, overrides map[string]string) string {
	actions := map[string]string{
		"/setup": "owner.created", "/api/v1/setup": "owner.created", "/login": "session.created", "/logout": "session.ended", "/login/oidc/callback": "sso.session",
		"/settings/server": "settings.server.updated", "/api/v1/settings/server": "settings.server.updated", "/settings/navigation": "settings.navigation.updated", "/api/v1/settings/navigation": "settings.navigation.updated", "/settings/mfa": "settings.mfa.updated", "/api/v1/settings/mfa": "settings.mfa.updated",
		"/settings/session-timeouts": "settings.session-timeouts.updated", "/api/v1/settings/session-timeouts": "settings.session-timeouts.updated",
		"/api/v1/updates":    "update.requested",
		"/settings/playback": "settings.playback.updated", "/api/v1/settings/playback": "settings.playback.updated",
		"/settings/transcoder": "settings.transcoder.updated", "/api/v1/settings/transcoder": "settings.transcoder.updated",
		"/settings/transcoder/test": "settings.transcoder.tested", "/api/v1/transcoder/test": "settings.transcoder.tested",
		"/settings/subtitles": "settings.subtitles.updated", "/api/v1/settings/subtitles": "settings.subtitles.updated",
		"/settings/scans": "settings.scans.updated", "/api/v1/settings/scans": "settings.scans.updated",
		"/settings/dlna": "settings.dlna.updated", "/api/v1/settings/dlna": "settings.dlna.updated",
		"/settings/jellyfin": "settings.jellyfin.updated", "/onboarding/jellyfin": "settings.jellyfin.updated", "/api/v1/settings/jellyfin": "settings.jellyfin.updated",
		"/settings/updates": "settings.updates.updated", "/onboarding/updates": "settings.updates.updated", "/api/v1/settings/updates": "settings.updates.updated",
		"/settings/updates/check": "updates.checked", "/onboarding/updates/check": "updates.checked", "/api/v1/updates/check": "updates.checked",
		"/settings/trusted-https": "settings.trusted-https.updated", "/onboarding/trusted-https": "settings.trusted-https.updated", "/api/v1/settings/trusted-https": map[bool]string{true: "settings.trusted-https.disabled", false: "settings.trusted-https.updated"}[request.Method == http.MethodDelete],
		"/settings/trusted-https/disable": "settings.trusted-https.disabled",
		"/settings/configuration":         "configuration.updated", "/settings/configuration/reset": "configuration.reset", "/onboarding/tmdb": "configuration.updated",
		"/settings/libraries": "library.added", "/settings/libraries/remove": "library.removed", "/api/v1/libraries": map[bool]string{true: "library.removed", false: "library.added"}[request.Method == http.MethodDelete],
		"/settings/profiles": "profile.created", "/settings/profiles/remove": "profile.removed", "/settings/profiles/password": "profile.password", "/settings/profiles/permissions": "profile.permissions",
		"/settings/sessions/revoke": "sessions.revoked", "/settings/sessions/device": "session.revoked", "/settings/api-keys": "api-key.created", "/settings/api-keys/revoke": "api-key.revoked", "/quick-connect": "quick-connect.approved",
		"/settings/tasks/scan": "task.library-scan", "/settings/tasks/metadata": "task.metadata-refresh", "/settings/tasks/maintain": "task.maintenance", "/settings/cache/clear": "task.cache-clear", "/scan": "task.library-scan",
		"/settings/encrypted-backup": "backup.created", "/settings/backups/verify": "backup.verified", "/settings/marker-analysis": "task.marker-analysis",
		"/account/mfa/setup": "mfa.setup", "/account/mfa/enable": "mfa.enabled", "/account/mfa/disable": "mfa.disabled", "/account/oidc/unlink": "oidc.unlinked", "/account/saml/unlink": "saml.unlinked",
		"/account/passkeys/remove": "passkey.removed",
		"/settings/media-shares":   "media-share.created", "/settings/remote/kill": "remote-access.killed", "/settings/remote/enable": "remote-access.reset",
	}
	for key, value := range overrides {
		if value == "" {
			delete(actions, key)
		} else {
			actions[key] = value
		}
	}
	return actions[request.URL.Path]
}

func playbackMutation(path string) bool {
	return strings.HasPrefix(path, "/progress/") || strings.HasPrefix(path, "/watched/") || strings.Contains(path, "/progress") ||
		strings.Contains(path, "PlayedItems") || path == "/Sessions/Playing" || strings.HasPrefix(path, "/Sessions/Playing/") ||
		path == "/Sessions/Capabilities" || path == "/Sessions/Capabilities/Full"
}

func apiAdministrativeChange(request *http.Request) bool { //nolint:cyclop // Administrative API prefixes are an explicit security boundary.
	path := request.URL.Path
	return strings.HasPrefix(path, "/api/v1/settings/") || strings.HasPrefix(path, "/api/v1/profiles") || strings.HasPrefix(path, "/api/v1/devices") ||
		strings.HasPrefix(path, "/api/v1/sessions") || strings.HasPrefix(path, "/api/v1/api-keys") || strings.HasPrefix(path, "/api/v1/tasks") ||
		strings.HasPrefix(path, "/api/v1/remote-access") || strings.Contains(path, "/metadata") || strings.Contains(path, "/subtitles") ||
		strings.HasPrefix(path, "/api/v1/collections") || strings.HasPrefix(path, "/api/v1/backups") || strings.HasPrefix(path, "/api/v1/updates")
}

func Target(request *http.Request) string {
	for _, key := range []string{"key", "name", "device", "path", "id"} {
		if target := strings.TrimSpace(request.FormValue(key)); target != "" {
			return Truncate(target, 120)
		}
	}
	return request.URL.Path
}

func Details(request *http.Request) map[string]string { //nolint:cyclop,gocognit // Form and JSON allowlisting share one redaction path.
	allowed := map[string]bool{
		"key": true, "name": true, "mode": true, "autoplay": true, "subtitles": true, "markers": true, "autoSkip": true,
		"quality": true, "codec": true, "accelerator": true, "toneMap": true, "language": true, "frequency": true, "enabled": true, "updateMode": true, "automatic": true,
		"path": true, "owner": true, "rating": true, "libraries": true, "start": true, "end": true, "remote": true,
		"transcode": true, "downloads": true, "listed": true, "included": true, "task": true,
		"label": true, "title": true, "year": true, "type": true,
	}
	values := make(map[string][]string)
	mediaType, _, _ := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if mediaType == "application/json" && request.Body != nil {
		data, err := io.ReadAll(request.Body)
		request.Body = io.NopCloser(bytes.NewReader(data))
		var input map[string]any
		if err == nil && json.Unmarshal(data, &input) == nil {
			for key, value := range input {
				values[key] = []string{stringValue(value)}
			}
		}
	} else {
		_ = request.ParseForm()
		values = request.Form
	}
	result := make(map[string]string)
	for key, items := range values {
		if allowed[key] && len(items) > 0 {
			value := strings.Join(items, ",")
			if key == "key" && SecretSetting(value) {
				result[key] = value
				result["value"] = "[redacted]"
			} else {
				result[key] = Truncate(value, 240)
			}
		}
	}
	switch request.URL.Path {
	case "/Users/AuthenticateByName":
		result["channel"], result["privileges"] = "jellyfin-compatibility", "media-only"
	case "/Users/AuthenticateWithQuickConnect":
		result["channel"], result["privileges"] = "jellyfin-quick-connect", "media-only"
	}
	return result
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, stringValue(item))
		}
		return strings.Join(parts, ",")
	default:
		return ""
	}
}

func SecretSetting(key string) bool {
	key = strings.ToLower(key)
	return strings.Contains(key, "password") || strings.Contains(key, "secret") || strings.Contains(key, "token") || strings.HasSuffix(key, ".key")
}

func Truncate(value string, limit int) string {
	return value[:min(len(value), limit)]
}
