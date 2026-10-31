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
	return auth.confirm(currentViewer(request), code)
}

func (auth *mfaAuth) verifyRequest(request *http.Request, code string) bool {
	return auth.profiles.verifySecondFactor(currentViewer(request).ID, code)
}

func (auth *mfaAuth) disableRequest(request *http.Request) error {
	return auth.profiles.disableMFA(currentViewer(request).ID)
}

var mfaSetupHTML = identitycore.MFASetupPageHTML("Kinosail Player", "Authentication code")

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
