package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/updatecontrol"
)

var (
	stepUpLoginPath  = identitycore.StepUpLoginPath
	passkeyOfferPath = identitycore.PasskeyOfferPath
)

func safeLoginReturn(raw string) string { return identitycore.SafeLoginReturn(raw) }

func (auth *authentication) onboardingLoginNext(profile viewerProfile, request *http.Request, next string) string {
	if profile.Owner && auth.settings.onboardingPending() && request.URL.Query().Get("stepup") != "1" && request.URL.Query().Get("next") == "" {
		return "/onboarding"
	}
	return next
}

func (auth *authentication) login(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop,gocognit // Password, MFA, and session decisions remain one auditable login flow.
	if (request.Method == http.MethodGet || request.Method == http.MethodHead) && publicInternetRequest(request) {
		auth.publicLogin(writer, request)
		return
	}
	if request.Method == http.MethodGet && auth.sso {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		view := profileLoginSSOView
		if auth.saml && !auth.oidc {
			view = profileLoginSAMLView
		} else if auth.saml {
			view = profileLoginFederatedView
		}
		_ = view.Execute(writer, request, safeLoginReturn(request.URL.Query().Get("next")))
		return
	}
	if request.Method == http.MethodGet {
		auth.profiles.login(writer, request, safeLoginReturn(request.URL.Query().Get("next")))
		return
	}
	if publicInternetRequest(request) {
		localizedPageError(writer, request, "Use a passkey to sign in.", "public password login is disabled; use a passkey or Quick Connect", http.StatusForbidden)
		return
	}
	if !auth.allowCredentialLogin(request.RemoteAddr, request.FormValue("name")) {
		writer.Header().Set("Retry-After", "60")
		localizedError(writer, request, "too many login attempts", http.StatusTooManyRequests)
		return
	}
	profile, ok := auth.profiles.authenticate(request.FormValue("name"), request.FormValue("password"))
	if !ok {
		localizedError(writer, request, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if profile.TOTPSecret != "" && !auth.profiles.verifySecondFactor(profile.ID, request.FormValue("code")) {
		localizedError(writer, request, "invalid authentication code", http.StatusUnauthorized)
		return
	}
	auth.credentialLoginSucceeded(request.FormValue("name"))
	if err := auth.profiles.signInStrong(writer, request, profile.ID); err != nil {
		localizedError(writer, request, "could not create session", http.StatusInternalServerError)
		return
	}
	setAuditViewer(request, profile)
	if (profile.Owner || auth.settings.requireMFA()) && !profile.Secured() {
		http.Redirect(writer, request, "/account?mfa=required", http.StatusSeeOther)
		return
	}
	next := safeLoginReturn(request.URL.Query().Get("next"))
	next = auth.onboardingLoginNext(profile, request, next)
	if request.URL.Query().Get("stepup") != "1" {
		next = passkeyOfferPath(next)
	}
	http.Redirect(writer, request, next, http.StatusSeeOther) //nolint:gosec // G710: safeLoginReturn rejects absolute, cross-origin, ambiguous, and oversized targets.
}

func (auth *authentication) allowLogin(remote string) bool {
	return auth.logins.Allow(httpguard.RemoteHost(remote), 10)
}

func (auth *authentication) allowCredentialLogin(remote, name string) bool {
	ip := auth.logins.Allow("ip:"+httpguard.RemoteHost(remote), 30)
	account := auth.logins.Allow("account:"+strings.ToLower(strings.TrimSpace(name)), 10)
	return ip && account
}

func (auth *authentication) credentialLoginSucceeded(name string) {
	auth.logins.Reset("account:" + strings.ToLower(strings.TrimSpace(name)))
}

func (auth *authentication) setup(writer http.ResponseWriter, request *http.Request) {
	if auth.profiles.hasProfiles() {
		http.Redirect(writer, request, "/", http.StatusSeeOther)
		return
	}
	if request.Method == http.MethodGet {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = setupView.Execute(writer, request, nil)
		return
	}
	automaticUpdates, err := updatecontrol.ParseMode(request, false, "name", "password", "totp")
	if err != nil || !validSetupForm(request) {
		if err == nil {
			err = errors.New("invalid Server setup")
		}
		localizedError(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	profile, enrollment, err := auth.createFirstOwner(request.FormValue("name"), request.FormValue("password"), request.FormValue("totp") == "true", automaticUpdates)
	if err != nil {
		localizedError(writer, request, err.Error(), ownerSetupStatus(err))
		return
	}
	if err := auth.profiles.signInStrong(writer, request, profile.ID); err != nil {
		localizedError(writer, request, "could not create session", http.StatusInternalServerError)
		return
	}
	setAuditViewer(request, profile)
	if enrollment != nil {
		auth.mfa.writeSetup(writer, request, *enrollment)
		return
	}
	http.Redirect(writer, request, "/account?setup=1", http.StatusSeeOther)
}

func validSetupForm(request *http.Request) bool {
	if len(request.PostForm["name"]) != 1 || len(request.PostForm["password"]) != 1 || len(request.PostForm["totp"]) > 1 {
		return false
	}
	totp := request.PostForm["totp"]
	return len(totp) == 0 || totp[0] == "true"
}
