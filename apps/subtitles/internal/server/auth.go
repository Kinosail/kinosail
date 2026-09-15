package server

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/navigation"
	"github.com/MikeO7/kinosail/packages/owneraccess"
)

type authentication struct {
	profiles *profileStore
	passkeys *passkeyAuth
	mfa      *mfaAuth
	settings *settingsStore
	required bool
	logins   httpguard.Limiter
	sso      bool
	oidc     bool
	saml     bool
	audit    *auditStore
	notify   *notificationAdapter
	setupMu  sync.Mutex
}

type (
	viewerContextKey          struct{}
	ownerAutomationContextKey struct{}
)

const publicSessionMaximumAge = 31 * 24 * time.Hour

func newAuthentication(ctx context.Context, dataDir string, required bool, authURL string, settings *settingsStore, notifications NotificationConfig, stateDB *database.Store, retentions ...time.Duration) *authentication { //nolint:contextcheck // Startup state loading must finish independently of lifecycle cancellation.
	notify := newNotification(notifications)
	profiles := newProfileStore(dataDir, stateDB) //nolint:contextcheck // Startup state loading must finish independently of lifecycle cancellation.
	profiles.sessionTimeouts = settings.sessionTimeouts
	auth := &authentication{profiles: profiles, passkeys: newPasskeyAuth(authURL, profiles), mfa: newMFA(profiles), settings: settings, required: required, audit: newAuditStore(ctx, dataDir, func(event auditEvent) { notify.send(ctx, event) }, retentions...), notify: notify}
	auth.passkeys.audit, auth.passkeys.settings = auth.audit, settings
	return auth
}

func (auth *authentication) protect(next http.Handler, pattern func(*http.Request) string) http.Handler { //nolint:cyclop,gocognit // Authentication decisions remain linear and auditable below the repository quality ceiling.
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !publicInternetRequest(request) && navigation.IsBrowserRequest(request, authenticationErrorRequest(request)) && !auth.passkeys.engine.RequirePageOrigin(writer, request, auth.passkeys.redirectPages && auth.passkeys.err == nil, secureRequest(request)) {
			return
		}
		if auth.profiles.err != nil {
			auth.audit.Denied(request, "profile storage unavailable")
			localizedError(writer, request, "profile storage is unavailable", http.StatusServiceUnavailable)
			return
		}
		matched := pattern(request)
		if publicInternetRequest(request) && remotePublicRouteDenied(matched) {
			auth.audit.Denied(request, "route unavailable remotely")
			localizedNotFound(writer, request)
			return
		}
		if (matched == "" || auth.public(request, matched)) && (owneraccess.ProfileID(request) == "" || identitycore.ManagementBootstrapRoute(matched)) {
			auth.audit.Track(next, writer, request)
			return
		}
		if profile, found := auth.identity(request); found {
			if bound := owneraccess.ProfileID(request); bound != "" && (profile.ID != bound || !profile.Owner || profile.APIKey || !profile.Secured() || auth.profiles.publicSession(request) || !auth.profiles.recentlyAuthenticated(request, 8*time.Hour)) {
				if authenticationErrorRequest(request) {
					localizedError(writer, request, "sign in as the Owner paired to this device using two-step sign-in", http.StatusForbidden)
				} else {
					http.Redirect(writer, request, stepUpLoginPath(request), http.StatusSeeOther)
				}
				return
			}
			setAuditViewer(request, profile)
			access := auth.viewerAccess(profile, request, matched)
			if access != identitycore.Allowed {
				auth.denyViewerAccess(writer, request, access)
				return
			}
			request = request.WithContext(context.WithValue(request.Context(), viewerContextKey{}, profile))
			auth.audit.Track(next, writer, request)
			return
		}
		if authenticationErrorRequest(request) {
			auth.audit.Denied(request, "authentication required")
			localizedError(writer, request, "authentication required", http.StatusUnauthorized)
			return
		}
		auth.redirectToSignIn(writer, request)
	})
}

func (auth *authentication) redirectToSignIn(writer http.ResponseWriter, request *http.Request) {
	signIn := auth.signInPath()
	if signIn == "/login" && request.Method == http.MethodGet && (request.URL.Path == "/oauth/authorize" || request.URL.Path == "/home-assistant/authorize") && len(request.URL.RequestURI()) <= 4096 {
		signIn += "?next=" + url.QueryEscape(request.URL.RequestURI())
	}
	http.Redirect(writer, request, signIn, http.StatusSeeOther)
}

func (auth *authentication) viewerAccess(profile viewerProfile, request *http.Request, matched string) identitycore.Denial {
	input := identitycore.AccessInput{
		EnrollmentRoute: mfaEnrollmentRoute(matched), Secured: profile.Secured(), MFARequired: auth.settings.requireMFA(), LocalOwner: profile.ID == "local-owner",
		APIKeyAllowed: profileAllowsAPI(profile, matched), ScheduleAllowed: profile.Allowed(publicInternetRequest(request), time.Now()), RemoteRouteAllowed: publicViewerRouteAllowed(matched),
		PublicSession: auth.profiles.publicSession(request), RecentlyAuthenticated: auth.profiles.recentlyAuthenticated(request, publicSessionMaximumAge),
		APIKey: profile.APIKey, Owner: profile.Owner, Public: publicInternetRequest(request),
	}
	return identitycore.EvaluateAccess(input)
}

func mfaEnrollmentRoute(pattern string) bool {
	return identitycore.MFAEnrollmentRoute(pattern)
}

func (auth *authentication) denyViewerAccess(writer http.ResponseWriter, request *http.Request, denial identitycore.Denial) {
	identitycore.RespondDenial(writer, request, denial, identitycore.DenialConfig{Denied: auth.audit.Denied, NotFound: localizedNotFound, AuthenticationError: authenticationErrorRequest, Error: localizedError, JSON: writeJSON})
}

func (auth *authentication) setRequireMFA(required bool) (bool, error) {
	if err := auth.settings.editable("security.require_mfa"); err != nil {
		return false, err
	}
	revoke := required && !auth.settings.requireMFA()
	if revoke {
		if err := auth.profiles.revokeAllSessions(); err != nil {
			return false, err
		}
	}
	return revoke, auth.settings.setRequireMFA(required)
}

func authenticationErrorRequest(request *http.Request) bool {
	return strings.HasPrefix(request.URL.Path, "/api/") || jellyfinPath(request.URL.Path)
}

func (auth *authentication) public(request *http.Request, pattern string) bool { //nolint:cyclop // Route capability policy remains one auditable decision.
	access := publicRoutes[pattern]
	if access == publicRoute {
		return true
	}
	if access == localCompatibilityRoute {
		return sessionToken(request) == ""
	}
	if access != capabilityRoute {
		return false
	}
	if pattern == "GET /Videos/{id}/{stream...}" {
		path := strings.TrimPrefix(request.URL.Path, "/Videos/")
		slash := strings.IndexByte(path, '/')
		if slash < 0 {
			return false
		}
		_, file, planned := plannedHLSFile(strings.ToLower(path[slash+1:]))
		return planned && hlsFile(file) && validPlaybackSession(jellyfinPlaySessionID(request))
	}
	if strings.HasPrefix(pattern, "GET /Videos/") || pattern == "GET /Audio/{id}/{stream}" {
		stream := strings.ToLower(request.URL.Path[strings.LastIndex(request.URL.Path, "/")+1:])
		if stream != "master.m3u8" && !strings.HasPrefix(stream, "stream") {
			return true
		}
		return jellyfinPlaySessionID(request) != ""
	}
	return true
}

func (auth *authentication) identity(request *http.Request) (viewerProfile, bool) {
	if profile, found := auth.profiles.profile(request); found {
		return profile, true
	}
	open := !auth.required && !auth.profiles.hasProfiles()
	return viewerProfile{ID: "local-owner", Name: "Owner", Owner: true}, open
}

func (auth *authentication) signInPath() string {
	if !auth.profiles.hasProfiles() {
		return "/setup"
	}
	return "/login"
}

func (auth *authentication) register(mux *http.ServeMux) {
	mux.HandleFunc("DELETE /api/v1/session", auth.deleteAPISession)
	mux.HandleFunc("POST /api/v1/setup", auth.createAPISetup)
	mux.HandleFunc("POST /api/v1/session", auth.createAPISession)
	mux.HandleFunc("POST /logout", auth.logout)
	mux.HandleFunc("POST /setup", auth.setup)
	mux.HandleFunc("GET /setup", auth.setup)
	mux.HandleFunc("POST /login", auth.login)
	mux.HandleFunc("GET /login", auth.login)
	auth.mfa.register(mux)
	auth.passkeys.register(mux, auth.allowLogin)
}

func registerIdentity(mux *http.ServeMux, auth *authentication, quickTTL time.Duration, oidcConfig OIDCConfig, samlConfig SAMLConfig, scimConfig SCIMConfig) *quickConnectBroker {
	sso := newOIDC(oidcConfig, auth.profiles)
	saml := newSAML(samlConfig, auth.profiles, sso)
	auth.oidc, auth.saml = sso.Configured(), saml.Configured()
	auth.sso = auth.oidc || auth.saml
	auth.register(mux)
	quickConnect := newQuickConnect(quickTTL)
	quickConnect.register(mux, auth.profiles)
	sso.Register(mux)
	saml.Register(mux)
	registerSCIM(mux, scimConfig, auth.profiles)
	return quickConnect
}

func (auth *authentication) logout(writer http.ResponseWriter, request *http.Request) {
	if err := auth.profiles.signOut(request); err != nil {
		localizedError(writer, request, "could not end session", http.StatusInternalServerError)
		return
	}
	cookie := sessionCookie("") //nolint:gosec // sessionCookie always sets Secure, HttpOnly, and Strict SameSite.
	if publicInternetRequest(request) {
		cookie = publicSessionCookie("")
	}
	cookie.MaxAge = -1
	http.SetCookie(writer, cookie)
	http.Redirect(writer, request, "/login", http.StatusSeeOther)
}

func (auth *authentication) owner(next http.Handler) http.Handler {
	return identitycore.Owner(next, identitycore.OwnerConfig{Identity: ownerIdentity, Managed: managedOwnerRequest, RecentlyAuthenticated: auth.profiles.recentlyAuthenticated, AuthenticationError: authenticationErrorRequest, StepUpPath: stepUpLoginPath, Error: localizedError, JSON: writeJSON})
}

func ownerIdentity(request *http.Request) (bool, bool) {
	viewer := currentViewer(request)
	return viewer.Owner, viewer.ID == "local-owner"
}

func managedOwnerRequest(request *http.Request) bool {
	managed, _ := request.Context().Value(ownerAutomationContextKey{}).(bool)
	return managed
}

func currentViewer(request *http.Request) viewerProfile {
	profile, _ := request.Context().Value(viewerContextKey{}).(viewerProfile)
	return profile
}
