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

func newAuthentication(ctx context.Context, dataDir string, required bool, authURL string, settings *settingsStore, notifications NotificationConfig, stateDB *database.Store, retentions ...time.Duration) *authentication {
	notify := newNotification(notifications)
	profiles := newProfileStore(dataDir, stateDB)
	profiles.sessionTimeouts = settings.sessionTimeouts
	profiles.publicSessionTimeouts = settings.publicSessionTimeouts
	profiles.publicSessionLifetime = settings.publicSessionLifetime
	settings.profiles = profiles
	profiles.requireMFA = settings.requireMFA
	auth := &authentication{profiles: profiles, passkeys: newPasskeyAuth(authURL, profiles), mfa: newMFA(profiles), settings: settings, required: required, audit: newAuditStore(ctx, dataDir, func(event auditEvent) { notify.send(ctx, event) }, retentions...), notify: notify}
	auth.passkeys.audit, auth.passkeys.settings = auth.audit, settings
	return auth
}

func (auth *authentication) protect(next http.Handler, pattern func(*http.Request) string) http.Handler {
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
		if (matched == "" || publicPattern(matched)) && (owneraccess.ProfileID(request) == "" || identitycore.ManagementBootstrapRoute(matched)) {
			auth.audit.Track(next, writer, request)
			return
		}
		if auth.serveIdentity(next, writer, request, matched) {
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
	if signIn == "/login" && request.Method == http.MethodGet && request.URL.Path == "/oauth/authorize" && len(request.URL.RequestURI()) <= 4096 {
		signIn += "?next=" + url.QueryEscape(request.URL.RequestURI())
	}
	http.Redirect(writer, request, signIn, http.StatusSeeOther)
}

func (auth *authentication) viewerAccess(access tokenAccess, request *http.Request, matched string) identitycore.Denial {
	profile := access.Profile
	input := identitycore.AccessInput{
		EnrollmentRoute: mfaEnrollmentRoute(matched), Secured: profile.Secured(), MFARequired: auth.settings.requireMFA(), LocalOwner: profile.ID == "local-owner",
		APIKeyAllowed: profileAllowsAPI(profile, matched), ScheduleAllowed: profile.Allowed(publicInternetRequest(request), time.Now()), RemoteRouteAllowed: publicViewerRouteAllowed(matched),
		PublicSession: auth.profiles.accessPublic(request, access), RecentlyAuthenticated: auth.profiles.accessRecentlyAuthenticated(request, access, auth.publicAuthenticationMaximumAge()),
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
	return strings.HasPrefix(request.URL.Path, "/api/")
}

func publicPattern(pattern string) bool {
	return publicRoutes[pattern]
}

func (auth *authentication) identity(request *http.Request) (tokenAccess, bool) {
	if access, found := auth.profiles.access(request); found {
		return access, true
	}
	open := !auth.required && !auth.profiles.hasProfiles()
	return tokenAccess{Profile: viewerProfile{ID: "local-owner", Name: "Owner", Owner: true}}, open
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

func registerIdentity(mux *http.ServeMux, auth *authentication, oidcConfig OIDCConfig, samlConfig SAMLConfig) {
	sso, saml := auth.configureFederation(oidcConfig, samlConfig)
	auth.register(mux)
	sso.Register(mux)
	saml.Register(mux)
}

func (auth *authentication) logout(writer http.ResponseWriter, request *http.Request) {
	if err := auth.profiles.signOut(request); err != nil {
		localizedError(writer, request, "could not end session", http.StatusInternalServerError)
		return
	}
	cookie := sessionCookie("", request) //nolint:gosec // sessionCookie always sets Secure, HttpOnly, and Strict SameSite.
	if publicInternetRequest(request) {
		cookie = publicSessionCookie("", request)
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

func (auth *authentication) serveIdentity(next http.Handler, writer http.ResponseWriter, request *http.Request, matched string) bool {
	access, found := auth.identity(request)
	if !found {
		return false
	}
	profile := access.Profile
	if bound := owneraccess.ProfileID(request); bound != "" && !auth.managementIdentityAllowed(access, request, bound) {
		if authenticationErrorRequest(request) {
			localizedError(writer, request, "sign in as the Owner paired to this device using two-step sign-in", http.StatusForbidden)
		} else {
			http.Redirect(writer, request, stepUpLoginPath(request), http.StatusSeeOther)
		}
		return true
	}
	setAuditViewer(request, profile)
	if denial := auth.viewerAccess(access, request, matched); denial != identitycore.Allowed {
		auth.denyViewerAccess(writer, request, denial)
		return true
	}
	auth.profiles.sessionModule().MigrateBrowserCookie(writer, request)
	request = request.WithContext(context.WithValue(request.Context(), viewerContextKey{}, profile))
	auth.audit.Track(next, writer, request)
	return true
}

func (auth *authentication) managementIdentityAllowed(access tokenAccess, request *http.Request, bound string) bool {
	profile := access.Profile
	return profile.ID == bound && profile.Owner && !profile.APIKey && profile.Secured() && !auth.profiles.accessPublic(request, access) && auth.profiles.accessRecentlyAuthenticated(request, access, 8*time.Hour)
}
