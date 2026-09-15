package identitycore

import (
	"net/http"
	"strings"
)

// Denial identifies one shared Viewer access rejection.
type Denial uint8

const (
	Allowed Denial = iota
	OwnerRemoteDenied
	StrongAuthenticationRequired
	PublicSessionRequired
	RemoteRouteDenied
	ViewerScheduleDenied
	APIKeyScopeDenied
	MFAEnrollmentRequired
	OwnerRequired
	OwnerStepUpRequired
)

// AccessInput contains evaluated app state for one Viewer request.
type AccessInput struct {
	Public, Owner, APIKey, RecentlyAuthenticated, PublicSession bool
	RemoteRouteAllowed, ScheduleAllowed, APIKeyAllowed          bool
	LocalOwner, MFARequired, Secured, EnrollmentRoute           bool
}

// EvaluateAccess applies Player's generic Viewer access policy in order.
func EvaluateAccess(input AccessInput) Denial { //nolint:cyclop,gocognit // The ordered policy stays explicit for security review.
	if input.Public {
		if input.Owner {
			return OwnerRemoteDenied
		}
		if input.APIKey {
			return OwnerRemoteDenied
		}
		if !input.RecentlyAuthenticated {
			return StrongAuthenticationRequired
		}
		if !input.PublicSession {
			return PublicSessionRequired
		}
		if !input.RemoteRouteAllowed {
			return RemoteRouteDenied
		}
	}
	if !input.ScheduleAllowed {
		return ViewerScheduleDenied
	}
	if input.APIKey {
		if !input.APIKeyAllowed {
			return APIKeyScopeDenied
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
		return MFAEnrollmentRequired
	}
	if input.MFARequired {
		return MFAEnrollmentRequired
	}
	return Allowed
}

// EvaluateOwner applies Player's Owner and recent-authentication policy.
func EvaluateOwner(owner, localOwner, managed, safeMethod, recentlyAuthenticated bool) Denial {
	if !owner {
		return OwnerRequired
	}
	if !localOwner && !managed && !safeMethod && !recentlyAuthenticated {
		return OwnerStepUpRequired
	}
	return Allowed
}

// Reason returns the stable audit reason for a denial.
func (denial Denial) Reason() string {
	switch denial {
	case OwnerRemoteDenied:
		return "Owner and API key access unavailable remotely"
	case StrongAuthenticationRequired:
		return "strong authentication required remotely"
	case PublicSessionRequired:
		return "public session required remotely"
	case RemoteRouteDenied:
		return "route unavailable remotely"
	case ViewerScheduleDenied:
		return "viewer access unavailable"
	case APIKeyScopeDenied:
		return "API key scope"
	case MFAEnrollmentRequired:
		return "MFA enrollment required"
	case Allowed, OwnerRequired, OwnerStepUpRequired:
		return "Owner access required"
	}
	return "Owner access required"
}

// Message returns the stable user message for a denial.
func (denial Denial) Message() string {
	switch denial {
	case OwnerRemoteDenied:
		return "Owner access is available only on the local network or WireGuard"
	case StrongAuthenticationRequired:
		return "strong authentication is required for public access"
	case PublicSessionRequired:
		return "public sign-in is required for public access"
	case ViewerScheduleDenied:
		return "Viewer access is not available"
	case APIKeyScopeDenied:
		return "API key scope does not allow this request"
	case MFAEnrollmentRequired:
		return "MFA enrollment required"
	case Allowed, RemoteRouteDenied, OwnerRequired, OwnerStepUpRequired:
		return "Owner access required"
	}
	return "Owner access required"
}

// Status returns the HTTP status for a denial.
func (denial Denial) Status() int {
	if denial == OwnerStepUpRequired || denial == OwnerRemoteDenied || denial == StrongAuthenticationRequired || denial == PublicSessionRequired || denial == ViewerScheduleDenied || denial == APIKeyScopeDenied || denial == MFAEnrollmentRequired || denial == OwnerRequired {
		return http.StatusForbidden
	}
	return http.StatusNotFound
}

// Secured reports whether a profile has an enrolled second factor.
func Secured(secret string, passkeyCount int) bool { return secret != "" || passkeyCount > 0 }

// MFAEnrollmentRoute reports whether a route can complete mandatory enrollment.
func MFAEnrollmentRoute(pattern string) bool {
	switch pattern {
	case "GET /account", "POST /account/mfa/setup", "POST /account/mfa/enable", "POST /auth/passkeys/register/begin", "POST /auth/passkeys/register/finish", "POST /logout", "DELETE /api/v1/session", "GET /api/v1/me", "POST /api/v1/me/mfa/setup", "PUT /api/v1/me/mfa", "POST /api/v1/passkeys/register/begin", "POST /api/v1/passkeys/register/finish":
		return true
	default:
		return false
	}
}

// APIRoutes maps app-specific routes to shared scope policy.
type APIRoutes struct {
	SessionOnly, JellyfinLibrary, Library, Write, Stream, Download map[string]bool
}

// AllowsAPI applies Player's closed API-key scope policy.
func AllowsAPI(scopes []string, pattern string, routes APIRoutes) bool {
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

// RouteSet creates an immutable-by-convention route lookup.
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
