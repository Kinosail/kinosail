package identitycore

import (
	"net/http"
	"strings"
)

type Denial uint8

const (
	Allowed Denial = iota
	ownerRemoteDenied
	strongAuthenticationRequired
	publicSessionRequired
	remoteRouteDenied
	viewerScheduleDenied
	apiKeyScopeDenied
	mfaEnrollmentRequired
	ownerRequired
	ownerStepUpRequired
)

type AccessInput struct {
	Public, Owner, APIKey, RecentlyAuthenticated, PublicSession bool
	RemoteRouteAllowed, ScheduleAllowed, APIKeyAllowed          bool
	LocalOwner, MFARequired, Secured, EnrollmentRoute           bool
}

func EvaluateAccess(input AccessInput) Denial {
	if input.Public {
		if input.Owner {
			return ownerRemoteDenied
		}
		if input.APIKey {
			return ownerRemoteDenied
		}
		if !input.RecentlyAuthenticated {
			return strongAuthenticationRequired
		}
		if !input.PublicSession {
			return publicSessionRequired
		}
		if !input.RemoteRouteAllowed {
			return remoteRouteDenied
		}
	}
	if !input.ScheduleAllowed {
		return viewerScheduleDenied
	}
	if input.APIKey {
		if !input.APIKeyAllowed {
			return apiKeyScopeDenied
		}
	}
	if input.LocalOwner {
		return Allowed
	}
	if input.Secured {
		return Allowed
	}
	if input.EnrollmentRoute {
		return Allowed
	}
	if input.Owner {
		return mfaEnrollmentRequired
	}
	if input.MFARequired {
		return mfaEnrollmentRequired
	}
	return Allowed
}

func evaluateOwner(owner, localOwner, managed, safeMethod, recentlyAuthenticated bool) Denial {
	if !owner {
		return ownerRequired
	}
	if !localOwner && !managed && !safeMethod && !recentlyAuthenticated {
		return ownerStepUpRequired
	}
	return Allowed
}

func (denial Denial) Reason() string {
	switch denial {
	case ownerRemoteDenied:
		return "Owner and API key access unavailable remotely"
	case strongAuthenticationRequired:
		return "strong authentication required remotely"
	case publicSessionRequired:
		return "public session required remotely"
	case remoteRouteDenied:
		return "route unavailable remotely"
	case viewerScheduleDenied:
		return "viewer access unavailable"
	case apiKeyScopeDenied:
		return "API key scope"
	case mfaEnrollmentRequired:
		return "MFA enrollment required"
	case Allowed, ownerRequired, ownerStepUpRequired:
		return "Owner access required"
	}
	return "Owner access required"
}

func (denial Denial) Message() string {
	switch denial {
	case ownerRemoteDenied:
		return "Owner access is available only on the local network or WireGuard"
	case strongAuthenticationRequired:
		return "strong authentication is required for public access"
	case publicSessionRequired:
		return "public sign-in is required for public access"
	case viewerScheduleDenied:
		return "Viewer access is not available"
	case apiKeyScopeDenied:
		return "API key scope does not allow this request"
	case mfaEnrollmentRequired:
		return "MFA enrollment required"
	case Allowed, remoteRouteDenied, ownerRequired, ownerStepUpRequired:
		return "Owner access required"
	}
	return "Owner access required"
}

func (denial Denial) Status() int {
	if denial == ownerStepUpRequired || denial == ownerRemoteDenied || denial == strongAuthenticationRequired || denial == publicSessionRequired || denial == viewerScheduleDenied || denial == apiKeyScopeDenied || denial == mfaEnrollmentRequired || denial == ownerRequired {
		return http.StatusForbidden
	}
	return http.StatusNotFound
}

func secured(secret string, passkeyCount int) bool { return secret != "" || passkeyCount > 0 }

func MFAEnrollmentRoute(pattern string) bool {
	switch pattern {
	case "GET /account", "POST /account/mfa/setup", "POST /account/mfa/enable", "POST /auth/passkeys/register/begin", "POST /auth/passkeys/register/finish", "POST /logout", "DELETE /api/v1/session", "GET /api/v1/me", "POST /api/v1/me/mfa/setup", "PUT /api/v1/me/mfa", "POST /api/v1/passkeys/register/begin", "POST /api/v1/passkeys/register/finish":
		return true
	default:
		return false
	}
}

type APIRoutes struct {
	SessionOnly, JellyfinLibrary, Library, Write, Stream, Download map[string]bool
}

func allowsAPI(scopes []string, pattern string, routes APIRoutes) bool {
	if hasScope(scopes, "home-assistant") {
		return strings.Contains(pattern, " /api/v1/home-assistant/")
	}
	if routes.SessionOnly[pattern] {
		return false
	}
	if pattern == "GET /api/v1/me" || pattern == "PUT /api/v1/me/language" {
		return hasScope(scopes, "library") || hasScope(scopes, "admin")
	}
	if routes.JellyfinLibrary[pattern] {
		return hasScope(scopes, "library")
	}
	scope := routeScope(pattern, routes)
	return scope != "" && hasScope(scopes, scope)
}

func RouteSet(patterns ...string) map[string]bool {
	result := make(map[string]bool, len(patterns))
	for _, pattern := range patterns {
		result[pattern] = true
	}
	return result
}

func routeScope(pattern string, routes APIRoutes) string {
	switch {
	case routes.Library[pattern]:
		return "library"
	case routes.Write[pattern]:
		return "write"
	case routes.Stream[pattern]:
		return "stream"
	case routes.Download[pattern]:
		return "download"
	case strings.Contains(pattern, " /api/v1/"):
		return "admin"
	default:
		return ""
	}
}

func hasScope(scopes []string, scope string) bool {
	for _, candidate := range scopes {
		if candidate == scope {
			return true
		}
	}
	return false
}
