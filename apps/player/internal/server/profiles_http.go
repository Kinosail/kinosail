package server

import (
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

const setupHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><meta name="theme-color" content="#0b0d0b"><link rel="manifest" href="/manifest.webmanifest"><link rel="icon" href="/static/icon.svg?v=8"><link rel="apple-touch-icon" href="/static/apple-touch-icon.png?v=8"><title>Set up Kinosail Player</title><link rel="stylesheet" href="/static/app.css"><script src="/static/theme.js"></script><script defer src="/static/main.kinosail.bundle.js"></script></head><body class="auth onboarding-page"><a class="skip" href="#main">Skip to content</a><main id="main" class="wizard-shell"><aside class="wizard-rail"><a class="wizard-brand" href="/setup"><img class="brand-mark" src="/static/icon.svg?v=8" alt=""><strong>Kinosail Player</strong></a><h2>Set up your Server</h2><ol class="wizard-steps"><li class="is-current" aria-current="step"><span>01</span><strong>Your account</strong><small>Name and password</small></li><li><span>02</span><strong>Your media</strong><small>Pick your movie and show folders</small></li><li><span>03</span><strong>Devices</strong><small>Watch on your TV and phone</small></li><li><span>04</span><strong>Household</strong><small>Add the people who watch</small></li><li><span>05</span><strong>Viewing history</strong><small>Optional import</small></li></ol><p class="wizard-privacy"><span class="wizard-dot"></span><strong>Local by default</strong><br>Your library, profiles, and viewing activity stay on this Server.</p></aside><section class="wizard-stage"><header class="wizard-header"><span class="eyebrow">Step 1 of 5 · Your account</span><h1>Welcome to Kinosail.</h1><p>Create your account. You use it to sign in and to manage this server. Setup takes about five minutes, and you can skip any step.</p></header><form class="wizard-form" method="post"><div class="wizard-fields"><label>Name<input id="setup-username" autofocus required name="name" maxlength="64" autocomplete="username" autocapitalize="none" spellcheck="false" placeholder="Your name"></label><label>Password<input id="new-password" required type="password" name="password" minlength="12" autocomplete="new-password" placeholder="At least 12 characters"><small>Use a password you can save in your password manager.</small></label></div><div class="wizard-callout"><span class="wizard-callout-mark">{{icon "check"}}</span><p><strong>Stored on your Server.</strong><br>Kinosail keeps the media catalog, profiles, and watch progress on this Server. Remote access is always a separate choice.</p></div><footer class="wizard-actions"><span>Next: protect your account, then add your movies and shows.</span><button>Create account &amp; continue</button></footer></form></section></main>{{languagePicker}}</body></html>`

const onboardingErrorHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><meta name="theme-color" content="#0b0d0b"><link rel="icon" href="/static/icon.svg?v=8"><title>Could not finish setup · Kinosail Player</title><link rel="stylesheet" href="/static/app.css"><script src="/static/theme.js"></script></head><body class="auth"><a class="skip" href="#main">Skip to content</a><main id="main" class="grant-card" aria-labelledby="onboarding-error-title"><img class="brand-mark" src="/static/icon.svg?v=8" alt=""><span class="eyebrow">Could not finish setup</span><h1 id="onboarding-error-title">Check your setup details.</h1><p role="alert">{{.Message}}</p><p>Your Owner account was not created. Check the details and try again.</p><a class="button" href="{{.ReturnPath}}">Try again</a></main>{{languagePicker}}</body></html>`

const profileLoginHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><meta name="theme-color" content="#0b0d0b"><link rel="manifest" href="/manifest.webmanifest"><link rel="icon" href="/static/icon.svg?v=8"><link rel="apple-touch-icon" href="/static/apple-touch-icon.png?v=8"><title>Sign in · Kinosail Player</title><link rel="stylesheet" href="/static/app.css"><script src="/static/theme.js"></script><script defer src="/static/main.kinosail.bundle.js"></script><script defer src="/static/passkeys.js?v=16"></script></head><body class="auth" data-login-next="{{.}}" data-passkey-verifying="{{t "Verifying your passkey…"}}" data-passkey-signed-in="{{t "Signed in. Opening your library…"}}" data-passkey-waiting="{{t "Waiting for your passkey…"}}" data-passkey-added="{{t "Passkey added."}}" data-passkey-failed="{{t "Passkey failed."}}" data-passkey-insecure="{{t "Passkeys need the configured Server address and trusted HTTPS. Open that address, trust the Server certificate, or use an authenticator."}}"><main><form method="post"><img class="brand-mark" src="/static/icon.svg?v=8" alt=""><span class="eyebrow">Welcome aboard</span><h1>Kinosail Player</h1><p>Sign in to your library.</p><button type="button" data-passkey-login>Sign in with passkey</button><label>Name<input id="username" autofocus required name="name" autocomplete="username webauthn" autocapitalize="none" spellcheck="false"></label><label>Password<input id="current-password" required type="password" name="password" autocomplete="current-password"></label><label>{{t "Authentication or recovery code"}} <small>{{t "if enabled"}}</small><input name="code" autocomplete="one-time-code"></label><button class="quiet">Sign in</button><output data-passkey-status></output></form>{{languagePicker}}</main></body></html>`

var (
	setupUpdates = `<details class="onboarding-disclosure wizard-update-disclosure"><summary>Update checks <span>Automatic by default</span></summary><fieldset class="wizard-update"><legend>Update checks</legend><label class="wizard-check"><input type="radio" name="updateMode" value="manual"><span><strong>Choose when to check</strong><small>Review signed updates yourself from Settings.</small></span></label><label class="wizard-check"><input type="radio" name="updateMode" value="automatic" checked><span><strong>Check automatically</strong><small>Kinosail checks about once a day and leaves installation under your control.</small></span></label></fieldset></details>`
	setupPage    = strings.NewReplacer(`<aside class="wizard-rail">`, `<aside class="wizard-rail" aria-label="Setup progress">`, `<ol class="wizard-steps">`, `<ol class="wizard-steps" tabindex="0" aria-label="Setup steps">`, `<aside class="wizard-aside">`, `<aside class="wizard-aside" aria-label="Setup summary">`).Replace(setupHTML)
	setupView    = newLazyLocalizedTemplate("setup", func() string {
		return strings.Replace(setupPage, `<div class="wizard-callout">`, setupUpdates+`<div class="wizard-callout">`, 1)
	})
	onboardingErrorView = newLocalizedTemplate("onboarding-error", onboardingErrorHTML)
	profileLoginView    = newLocalizedTemplate("login", profileLoginHTML)
	profileLoginSSOView = newLazyLocalizedTemplate("login-sso", func() string {
		return strings.Replace(profileLoginHTML, "</form>", `<a href="/api/v1/session/oidc">Sign in with SSO</a></form>`, 1)
	})
	profileLoginSAMLView = newLazyLocalizedTemplate("login-saml", func() string {
		return strings.Replace(profileLoginHTML, "</form>", `<a href="/api/v1/session/saml">Sign in with SAML</a></form>`, 1)
	})
	profileLoginFederatedView = newLazyLocalizedTemplate("login-federated", func() string {
		return strings.Replace(profileLoginHTML, "</form>", `<a href="/api/v1/session/oidc">Sign in with OpenID Connect</a><a href="/api/v1/session/saml">Sign in with SAML</a></form>`, 1)
	})
)

func (store *profileStore) login(writer http.ResponseWriter, request *http.Request, next string) {
	identitycore.Login(writer, request, next, profileLoginView.Execute, identitycore.PasswordLoginConfig[viewerProfile]{Authenticate: store.authenticate, ProfileID: func(profile viewerProfile) string { return profile.ID }, SignIn: store.signIn, SetAudit: setAuditViewer, ReturnPath: safeLoginReturn, Error: localizedError})
}

func resetViewerPassword(profiles *profileStore) http.HandlerFunc {
	return identitycore.ViewerPasswordResetHandler(profiles.resetPassword, localizedError)
}

func removeViewer(profiles *profileStore) http.HandlerFunc {
	return identitycore.ViewerRemovalHandler(profiles.removeProfile, localizedError)
}

func addViewer(profiles *profileStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		keys := []string{"name", "password", "owner", "rating", "libraries", "start", "end", "remote", "transcode", "downloads"}
		if request.URL.Path == "/onboarding/household" {
			keys = append(keys, "kind")
		}
		if !strictSingleValueForm(request, keys...) {
			localizedError(writer, request, "invalid Viewer Profile", http.StatusBadRequest)
			return
		}
		if _, err := profiles.addProfile(request.FormValue("name"), request.FormValue("password"), request.FormValue("owner") == "true", identitycore.ProfilePolicyFromRequest(request)); err != nil {
			localizedError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		if request.URL.Path == "/onboarding/household" {
			http.Redirect(writer, request, "/onboarding/household", http.StatusSeeOther)
			return
		}
		http.Redirect(writer, request, "/settings", http.StatusSeeOther)
	}
}

func addOnboardingViewer(profiles *profileStore) http.HandlerFunc {
	return identitycore.OnboardingViewerHandler(profiles.addProfile, localizedError)
}

func setViewerPermissions(profiles *profileStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := profiles.setProfile(request.FormValue("id"), request.FormValue("owner") == "true", identitycore.ProfilePolicyFromRequest(request)); err != nil {
			localizedError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(writer, request, "/settings", http.StatusSeeOther)
	}
}
