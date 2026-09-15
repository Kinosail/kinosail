package auditjournal

import (
	"net/http"
	"strings"
)

func dynamicAuditAction(request *http.Request) string {
	for _, classify := range []func(*http.Request) string{identityAuditAction, libraryAuditAction, accountAuditAction} {
		if action := classify(request); action != "" {
			return action
		}
	}
	return ""
}

func identityAuditAction(request *http.Request) string { //nolint:cyclop // The route vocabulary remains explicit for audit review.
	path := request.URL.Path
	switch {
	case strings.HasPrefix(path, "/scim/v2/Users"):
		return map[string]string{http.MethodPost: "scim.profile.provisioned", http.MethodPut: "scim.profile.provisioned", http.MethodPatch: "scim.profile.provisioned", http.MethodDelete: "scim.profile.deprovisioned"}[request.Method]
	case strings.HasPrefix(path, "/api/v1/profiles"):
		if strings.HasSuffix(path, "/password") {
			return "profile.password"
		}
		return map[string]string{http.MethodPost: "profile.created", http.MethodPut: "profile.permissions", http.MethodDelete: "profile.removed"}[request.Method]
	case strings.HasPrefix(path, "/api/v1/api-keys"):
		return map[string]string{http.MethodPost: "api-key.created", http.MethodDelete: "api-key.revoked"}[request.Method]
	case strings.HasPrefix(path, "/api/v1/devices"):
		return "session.revoked"
	case strings.HasPrefix(path, "/api/v1/sessions"):
		return "sessions.revoked"
	case strings.HasPrefix(path, "/api/v1/tasks/"):
		return "task." + strings.TrimPrefix(path, "/api/v1/tasks/")
	case strings.HasPrefix(path, "/api/v1/backups"):
		if strings.HasSuffix(path, "/verify") {
			return "backup.verified"
		}
		return "backup.created"
	case strings.Contains(path, "/remote-access/wireguard"):
		return map[string]string{http.MethodPost: "wireguard-peer.created", http.MethodDelete: "wireguard-peer.revoked"}[request.Method]
	default:
		return map[string]string{
			http.MethodPost + " /api/v1/remote-access/kill":   "remote-access.killed",
			http.MethodDelete + " /api/v1/remote-access/kill": "remote-access.reset",
		}[request.Method+" "+path]
	}
}

func libraryAuditAction(request *http.Request) string { //nolint:cyclop // The route vocabulary remains explicit for audit review.
	path := request.URL.Path
	if strings.Contains(path, "/items/") {
		if strings.HasSuffix(path, "/list") {
			return "list.updated"
		}
	}
	switch {
	case strings.HasPrefix(path, "/api/v1/media-shares"), strings.HasPrefix(path, "/settings/media-shares/"):
		return mediaShareAction(request)
	case strings.Contains(path, "/metadata"):
		if strings.HasSuffix(path, "/refresh") {
			return "metadata.refreshed"
		}
		return "metadata.updated"
	case strings.Contains(path, "/markers"):
		return map[string]string{http.MethodPost: "marker.updated", http.MethodPut: "marker.updated", http.MethodDelete: "marker.removed"}[request.Method]
	case strings.Contains(path, "/subtitles"):
		return "subtitle.fetched"
	case strings.HasPrefix(path, "/api/v1/collections"), strings.HasPrefix(path, "/collection"):
		return collectionAction(request.Method)
	case strings.Contains(path, "/playlists"), strings.HasPrefix(path, "/playlist"):
		return playlistAction(request.Method)
	case strings.HasPrefix(path, "/offline/"), strings.Contains(path, "/downloads"):
		return downloadAction(request.Method)
	case strings.HasPrefix(path, "/list/"):
		return "list.updated"
	default:
		return ""
	}
}

func accountAuditAction(request *http.Request) string {
	path := request.URL.Path
	if strings.HasPrefix(path, "/api/v1/passkeys/") {
		if request.Method == http.MethodDelete {
			return "passkey.removed"
		}
	}
	if strings.Contains(path, "/oidc") {
		if request.Method == http.MethodDelete {
			return "oidc.unlinked"
		}
	}
	switch {
	case strings.Contains(path, "/passkeys/register"):
		return "passkey.registered"
	case strings.Contains(path, "/mfa"):
		return map[string]string{http.MethodPut: "mfa.enabled", http.MethodDelete: "mfa.disabled"}[request.Method]
	default:
		return ""
	}
}

func mediaShareAction(request *http.Request) string {
	path := request.URL.Path
	if strings.HasSuffix(path, "/claim") {
		return "media-share.claimed"
	}
	if strings.HasSuffix(path, "/revoke") {
		return "media-share.revoked"
	}
	return map[string]string{http.MethodPost: "media-share.created", http.MethodDelete: "media-share.revoked"}[request.Method]
}

func collectionAction(method string) string {
	return map[string]string{http.MethodPost: "collection.updated", http.MethodPut: "collection.updated", http.MethodDelete: "collection.removed"}[method]
}

func playlistAction(method string) string {
	return map[string]string{http.MethodPost: "playlist.updated", http.MethodPut: "playlist.updated", http.MethodDelete: "playlist.removed"}[method]
}

func downloadAction(method string) string {
	return map[string]string{http.MethodPost: "download.created", http.MethodDelete: "download.removed"}[method]
}
