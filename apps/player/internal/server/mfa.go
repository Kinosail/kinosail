package server

import (
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

type mfaEnrollment = identitycore.Enrollment

type mfaAuth struct {
	profiles *profileStore
	engine   *identitycore.MFA
}

func newMFA(profiles *profileStore) *mfaAuth {
	return &mfaAuth{profiles: profiles, engine: identitycore.NewMFA()}
}

func (auth *mfaAuth) setup(profile viewerProfile) (mfaEnrollment, error) {
	return auth.engine.Setup(profile.ID, profile.Name)
}

func (auth *mfaAuth) discard(profileID string) {
	auth.engine.Discard(profileID)
}

func (auth *mfaAuth) confirm(profile viewerProfile, code string) error {
	enrollment, err := auth.engine.Confirm(profile.ID, code)
	if err != nil {
		return err
	}
	return auth.profiles.enableMFA(profile.ID, enrollment.Secret, enrollment.RecoveryCodes)
}

func (auth *mfaAuth) register(mux *http.ServeMux) {
	handlers, err := identitycore.NewMFAHandlers(identitycore.MFAHTTPConfig{
		RecentlyAuthenticated: auth.profiles.recentlyAuthenticated,
		ReadJSON:              readJSON, Setup: auth.setupRequest, Confirm: auth.confirmRequest, MarkStrong: auth.profiles.markStrong,
		Verify: auth.verifyRequest, Disable: auth.disableRequest, Error: apiError, JSON: writeJSON,
		WebError: func(writer http.ResponseWriter, request *http.Request, err error, status int) {
			localizedError(writer, request, err.Error(), status)
		},
		WebConfirmSuccess: func(writer http.ResponseWriter, request *http.Request) {
			next := "/account"
			if request.FormValue("next") == "/onboarding/connection" {
				next = "/onboarding/connection"
			}
			http.Redirect(writer, request, next, http.StatusSeeOther)
		},
		WebDisableSuccess: func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(writer, request, "/account", http.StatusSeeOther)
		},
	})
	if err != nil {
		panic(err)
	}
	mux.HandleFunc("POST /api/v1/me/mfa/setup", handlers.Setup)
	mux.HandleFunc("PUT /api/v1/me/mfa", handlers.Confirm)
	mux.HandleFunc("DELETE /api/v1/me/mfa", handlers.Disable)
	mux.HandleFunc("POST /account/mfa/setup", auth.webSetup)
	mux.HandleFunc("POST /account/mfa/enable", handlers.WebConfirm)
	mux.HandleFunc("POST /account/mfa/disable", handlers.WebDisable)
}

func (auth *mfaAuth) setupRequest(request *http.Request) (mfaEnrollment, error) {
	return auth.setup(currentViewer(request))
}

func (auth *mfaAuth) confirmRequest(request *http.Request, code string) error {
	return auth.confirm(currentViewer(request), code) //nolint:contextcheck // Factor changes must finish after authenticator confirmation.
}

func (auth *mfaAuth) verifyRequest(request *http.Request, code string) bool {
	return auth.profiles.verifySecondFactor(currentViewer(request).ID, code)
}

func (auth *mfaAuth) disableRequest(request *http.Request) error {
	return auth.profiles.disableMFA(currentViewer(request).ID) //nolint:contextcheck // Factor changes must finish after validation.
}

const mfaSetupHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Add extra sign-in protection · Kinosail Player</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=skeleton-2"></head><body class="auth"><main class="grant-card"><h1>Add extra sign-in protection</h1><p>Open your authenticator app and add Kinosail. Enter the setup key below, or use the setup link if your app supports it.</p><p><strong>Setup key</strong><br><code>{{.Secret}}</code></p><p><strong>Setup link</strong><br><code>{{.URI}}</code></p><h2>Recovery codes</h2>{{range .RecoveryCodes}}<code>{{.}}</code><br>{{end}}<p>Store these codes somewhere safe. Each works once.</p><form action="/account/mfa/enable" method="post">{{if .Next}}<input type="hidden" name="next" value="{{.Next}}">{{end}}<label>{{t "Authentication code"}}<input name="code" autocomplete="one-time-code" inputmode="numeric" pattern="[0-9]{6}" maxlength="6" required></label><button>Turn on extra sign-in protection</button></form></main></body></html>`

var mfaSetupView = newLocalizedTemplate("mfa-setup", mfaSetupHTML)

type mfaSetupPage struct {
	mfaEnrollment
	Next string
}

func (auth *mfaAuth) webSetup(writer http.ResponseWriter, request *http.Request) {
	if !auth.profiles.recentlyAuthenticated(request, 10*time.Minute) {
		http.Redirect(writer, request, stepUpLoginPath(request), http.StatusSeeOther)
		return
	}
	enrollment, err := auth.setup(currentViewer(request))
	if err != nil {
		localizedError(writer, request, "could not create MFA enrollment", http.StatusInternalServerError)
		return
	}
	auth.writeSetup(writer, request, enrollment)
}

func (auth *mfaAuth) writeSetup(writer http.ResponseWriter, request *http.Request, enrollment mfaEnrollment) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	next := ""
	if request.URL.Path == "/setup" || request.FormValue("next") == "/onboarding/connection" {
		next = "/onboarding/connection"
	}
	_ = mfaSetupView.Execute(writer, request, mfaSetupPage{enrollment, next})
}

func enrollmentJSON(enrollment mfaEnrollment) map[string]any {
	return map[string]any{"secret": enrollment.Secret, "uri": enrollment.URI, "recoveryCodes": enrollment.RecoveryCodes}
}
