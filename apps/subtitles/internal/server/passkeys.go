package server

import (
	"errors"
	"net/http"
	"strings"
	"time"

	sharedpasskeys "github.com/MikeO7/kinosail/packages/passkeys"
	"github.com/go-webauthn/webauthn/webauthn"
)

const accountHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Account · Kinosail Subtitles</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"><script defer src="/static/passkeys.js?v=14"></script></head><body class="auth" data-passkey-verifying="{{t "Verifying your passkey…"}}" data-passkey-signed-in="{{t "Signed in. Opening your library…"}}" data-passkey-waiting="{{t "Waiting for your passkey…"}}" data-passkey-added="{{t "Passkey added."}}" data-passkey-failed="{{t "Passkey failed."}}" data-passkey-insecure="{{t "Passkeys need the configured Server address and trusted HTTPS. Open that address, trust the Server certificate, or use an authenticator."}}"><main class="grant-card account-card"><img class="brand-mark" src="/static/icon.svg?v=11" alt=""><span class="eyebrow">Local account</span><h1>Ways to sign in</h1><p>Your password continues to work without internet access.</p><button type="button" data-passkey-add>{{if .HasPasskeys}}Add another passkey{{else}}Add a passkey{{end}}</button><p>The browser can save it on this device, use a nearby phone or tablet, or use a security key.</p><details class="passkey-device"><summary>Add one directly on another device</summary><p>On that device, open <code>{{.AccountURL}}</code>, sign in, then choose Add {{if .HasPasskeys}}another {{end}}passkey.</p></details><output data-passkey-status></output><h2>Passkeys</h2><p>See when each passkey was last used. Remove any passkey you no longer recognize or use.</p><div class="passkey-list">{{range .Passkeys}}<article><div><strong>Passkey {{.Display}}</strong>{{if .MostRecent}}<small class="passkey-recent">Most recently used</small>{{end}}<p>{{if .UsageTracked}}{{if .LastUsedLabel}}Last used {{.LastUsedLabel}}{{else}}Never used{{end}}{{else}}Last use is unknown{{end}}</p><p>{{if .BackupEligible}}{{if .BackedUp}}Available on your synced devices{{else}}Can sync across your devices{{end}}{{else}}Saved only on this device{{end}}</p>{{if .CloneWarning}}<p class="passkey-warning">This passkey may have been copied. Remove it if you do not recognize it.</p>{{end}}</div><form action="/account/passkeys/remove" method="post"><input type="hidden" name="id" value="{{.ID}}"><button class="danger">Remove passkey</button></form></article>{{else}}<p>No passkeys added yet.</p>{{end}}</div><h2>Extra sign-in protection</h2><p>Use an authenticator app as a second step when you sign in.</p><form action="/account/mfa/setup" method="post"><button>Set up an authenticator app</button></form><form action="/account/mfa/disable" method="post"><label>6-digit code<input name="code" autocomplete="one-time-code" required></label><button>Turn off extra sign-in protection</button></form><h2>Organization sign-in</h2><p>Use a work or school account to sign in to Kinosail.</p><a class="mode" href="/api/v1/me/oidc/link">Connect OpenID Connect</a><form action="/account/oidc/unlink" method="post"><button>Disconnect OpenID Connect</button></form><a class="mode" href="/api/v1/me/saml/link">Connect SAML</a><form action="/account/saml/unlink" method="post"><button>Disconnect SAML</button></form><p><a class="mode" href="/">Back to library</a></p>{{languagePicker}}</main></body></html>`

//nolint:gosec // G101: this is an account-security HTML template, not a credential.
const passkeyOfferHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Passkey sign-in · Kinosail Subtitles</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"><script defer src="/static/passkeys.js?v=14"></script></head><body class="auth" data-login-next="{{.Next}}" data-passkey-verifying="{{t "Verifying your passkey…"}}" data-passkey-signed-in="{{t "Signed in. Opening your library…"}}" data-passkey-waiting="{{t "Waiting for your passkey…"}}" data-passkey-added="{{t "Passkey added."}}" data-passkey-failed="{{t "Passkey failed."}}" data-passkey-insecure="{{t "Passkeys need the configured Server address and trusted HTTPS. Open that address, trust the Server certificate, or use an authenticator."}}"><main class="grant-card"><img class="brand-mark" src="/static/icon.svg?v=11" alt=""><span class="eyebrow">Faster sign-in</span>{{if .HasPasskeys}}<h1>A passkey is already set up</h1><p>You signed in another way. Use your existing passkey next time, or add another for this device.</p><button type="button" data-passkey-add>Add another passkey</button>{{else}}<h1>Make the next sign-in easier</h1><p>No passkey is saved for this account yet. Add one without replacing your password.</p><button type="button" data-passkey-add>Add a passkey</button>{{end}}<p>The browser can save it on this device, use a nearby phone or tablet, or use a security key.</p><details class="passkey-device"><summary>Add one directly on another device</summary><p>On that device, open <code>{{.AccountURL}}</code>, sign in, then choose Add {{if .HasPasskeys}}another {{end}}passkey.</p></details><output data-passkey-status></output><p><a class="mode passkey-skip" href="{{.Next}}">Not now</a></p></main></body></html>`

//nolint:gosec // G101: this is an account-security HTML template, not a credential.
const passkeyPromptHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Secure your account · Kinosail Subtitles</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"><script defer src="/static/passkeys.js?v=14"></script></head><body class="auth" data-passkey-verifying="{{t "Verifying your passkey…"}}" data-passkey-signed-in="{{t "Signed in. Opening your library…"}}" data-passkey-waiting="{{t "Waiting for your passkey…"}}" data-passkey-added="{{t "Passkey added."}}" data-passkey-failed="{{t "Passkey failed."}}" data-passkey-insecure="{{t "Passkeys need the configured Server address and trusted HTTPS. Open that address, trust the Server certificate, or use an authenticator."}}"><main class="grant-card"><img class="brand-mark" src="/static/icon.svg?v=11" alt=""><span class="eyebrow">Secure your account</span><h1>Protect the Owner account</h1><p>Every Owner must add a passkey or authenticator app before using Kinosail.</p><details class="wizard-help" open><summary>Choose a sign-in method</summary><ol><li>Create a passkey when this device supports biometrics, a device PIN, or a security key.</li><li>Use an authenticator app when you prefer a rotating code.</li><li>Keep your password as a recovery path for local sign-in.</li></ol></details><button type="button" data-passkey-add>Create passkey</button><output data-passkey-status></output><form action="/account/mfa/setup" method="post"><input type="hidden" name="next" value="/onboarding/connection"><button>Use an authenticator app instead</button></form>{{languagePicker}}</main></body></html>`

const mfaRequiredHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Extra sign-in protection required · Kinosail Subtitles</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"><script defer src="/static/passkeys.js?v=14"></script></head><body class="auth" data-passkey-verifying="{{t "Verifying your passkey…"}}" data-passkey-signed-in="{{t "Signed in. Opening your library…"}}" data-passkey-waiting="{{t "Waiting for your passkey…"}}" data-passkey-added="{{t "Passkey added."}}" data-passkey-failed="{{t "Passkey failed."}}" data-passkey-insecure="{{t "Passkeys need the configured Server address and trusted HTTPS. Open that address, trust the Server certificate, or use an authenticator."}}"><main class="grant-card"><img class="brand-mark" src="/static/icon.svg?v=11" alt=""><span class="eyebrow">Secure your account</span><h1>Extra sign-in protection is required</h1><p>Add a passkey or authenticator app. One is enough for the Owner account.</p><button type="button" data-passkey-add>Create passkey</button><output data-passkey-status></output><form action="/account/mfa/setup" method="post"><button>Set up an authenticator app</button></form><form action="/logout" method="post"><button class="quiet">Sign out</button></form></main></body></html>`

var (
	accountView       = newLocalizedTemplate("account", accountHTML)
	passkeyOfferView  = newLocalizedTemplate("passkey-offer", passkeyOfferHTML)
	passkeyPromptView = newLocalizedTemplate("passkey-prompt", strings.Replace(passkeyPromptHTML, `/static/app.css?v=electric-1`, `/static/app.css?v=electric-1`, 1))
	mfaRequiredView   = newLocalizedTemplate("mfa-required", mfaRequiredHTML)
)

type passkeyAuth struct {
	engine        *sharedpasskeys.Engine
	flow          *sharedpasskeys.ProfileFlow[viewerProfile]
	profiles      *profileStore
	settings      *settingsStore
	audit         *auditStore
	origin        string
	redirectPages bool
	err           error
}

type passkeySummary = sharedpasskeys.Summary

var (
	errInvalidPasskeyID = sharedpasskeys.ErrInvalidID
	errPasskeyNotFound  = sharedpasskeys.ErrNotFound
	errLastOwnerFactor  = sharedpasskeys.ErrLastFactor
)

func newPasskeyAuth(rawURL string, profiles *profileStore) *passkeyAuth {
	engine, err := sharedpasskeys.New(sharedpasskeys.Config{URL: rawURL, DefaultURL: "http://localhost:38128", DisplayName: "Kinosail", CookieName: "kinosail_passkey"})
	origin, redirectPages := "", false
	if engine != nil {
		origin, redirectPages = engine.Origin().String(), engine.Origin().RedirectPages()
	}
	auth := &passkeyAuth{engine: engine, profiles: profiles, origin: origin, redirectPages: redirectPages, err: err}
	if err == nil {
		auth.flow, auth.err = sharedpasskeys.NewProfileFlow(sharedpasskeys.ProfileFlowConfig[viewerProfile]{
			Engine: engine, Current: currentViewer, RecentlyAuthenticated: profiles.recentlyAuthenticated,
			Identity: subtitlePasskeyIdentity, Owner: subtitlePasskeyIsOwner, Remote: subtitlePasskeyIsRemote,
			PublicRequest: publicInternetRequest, SecureRequest: secureRequest,
			AddCredential: profiles.addPasskey, MarkStrong: profiles.markStrong, Discover: profiles.discoverPasskey, UpdateCredential: profiles.updatePasskey,
			SignIn: auth.signInPasskey, AfterLogin: auth.completePasskeyLogin, OnboardingNext: auth.passkeyOnboardingNext,
			WriteError: apiError,
		})
	}
	return auth
}

func (auth *passkeyAuth) register(mux *http.ServeMux, allowLogin func(string) bool) {
	mux.HandleFunc("GET /static/passkeys.js", serveScript(passkeysJS))
	mux.HandleFunc("GET /account", auth.account)
	mux.HandleFunc("POST /api/v1/passkeys/register/begin", auth.beginRegistration)
	mux.HandleFunc("POST /api/v1/passkeys/register/finish", auth.finishRegistration)
	mux.HandleFunc("POST /auth/passkeys/register/begin", auth.beginRegistration)
	mux.HandleFunc("POST /auth/passkeys/register/finish", auth.finishRegistration)
	mux.HandleFunc("GET /api/v1/passkeys", auth.listAPI)
	mux.HandleFunc("DELETE /api/v1/passkeys/{id}", auth.removeAPI)
	mux.HandleFunc("POST /account/passkeys/remove", auth.removeWeb)
	mux.HandleFunc("POST /api/v1/passkeys/login/begin", func(writer http.ResponseWriter, request *http.Request) {
		if !allowLogin(request.RemoteAddr) {
			writer.Header().Set("Retry-After", "60")
			apiError(writer, errors.New("too many login attempts"), http.StatusTooManyRequests)
			return
		}
		auth.beginLogin(writer, request)
	})
	mux.HandleFunc("POST /api/v1/passkeys/login/finish", auth.finishLogin)
	mux.HandleFunc("POST /auth/passkeys/login/begin", func(writer http.ResponseWriter, request *http.Request) {
		if !allowLogin(request.RemoteAddr) {
			writer.Header().Set("Retry-After", "60")
			apiError(writer, errors.New("too many login attempts"), http.StatusTooManyRequests)
			return
		}
		auth.beginLogin(writer, request)
	})
	mux.HandleFunc("POST /auth/passkeys/login/finish", auth.finishLogin)
}

func (auth *passkeyAuth) removeAPI(writer http.ResponseWriter, request *http.Request) {
	if !auth.profiles.recentlyAuthenticated(request, 10*time.Minute) {
		writeJSON(writer, map[string]any{"error": "recent authentication required", "stepUpRequired": true}, http.StatusForbidden)
		return
	}
	if err := auth.profiles.removePasskey(currentViewer(request).ID, request.PathValue("id")); err != nil { //nolint:contextcheck // An authorized credential removal must finish despite client cancellation.
		apiError(writer, err, passkeyRemovalStatus(err))
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (auth *passkeyAuth) removeWeb(writer http.ResponseWriter, request *http.Request) {
	if !auth.profiles.recentlyAuthenticated(request, 10*time.Minute) {
		http.Redirect(writer, request, stepUpLoginPath(request), http.StatusSeeOther)
		return
	}
	if err := auth.profiles.removePasskey(currentViewer(request).ID, request.FormValue("id")); err != nil { //nolint:contextcheck // An authorized credential removal must finish despite client cancellation.
		localizedError(writer, request, err.Error(), passkeyRemovalStatus(err))
		return
	}
	http.Redirect(writer, request, "/account", http.StatusSeeOther)
}

func passkeyRemovalStatus(err error) int {
	switch {
	case errors.Is(err, errInvalidPasskeyID):
		return http.StatusBadRequest
	case errors.Is(err, errPasskeyNotFound):
		return http.StatusNotFound
	case errors.Is(err, errLastOwnerFactor):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func (auth *passkeyAuth) beginRegistration(writer http.ResponseWriter, request *http.Request) {
	if auth.err != nil {
		apiError(writer, errors.New("passkey registration is unavailable"), http.StatusServiceUnavailable)
		return
	}
	auth.flow.BeginRegistration(writer, request)
}

func (auth *passkeyAuth) finishRegistration(writer http.ResponseWriter, request *http.Request) {
	if auth.flow == nil {
		apiError(writer, errors.New("passkey ceremony expired"), http.StatusBadRequest)
		return
	}
	auth.flow.FinishRegistration(writer, request)
}

func (auth *passkeyAuth) beginLogin(writer http.ResponseWriter, request *http.Request) {
	if auth.err != nil {
		apiError(writer, errors.New("passkey login is unavailable"), http.StatusServiceUnavailable)
		return
	}
	auth.flow.BeginLogin(writer, request)
}

func (auth *passkeyAuth) finishLogin(writer http.ResponseWriter, request *http.Request) {
	if auth.flow == nil {
		apiError(writer, errors.New("passkey ceremony expired"), http.StatusBadRequest)
		return
	}
	auth.flow.FinishLogin(writer, request)
}

func (auth *passkeyAuth) signInPasskey(writer http.ResponseWriter, request *http.Request, id string, public bool) error {
	if public {
		return auth.profiles.signInStrongPublic(writer, request, id)
	}
	return auth.profiles.signInStrong(writer, request, id)
}

func (auth *passkeyAuth) completePasskeyLogin(request *http.Request, profile viewerProfile, credential *webauthn.Credential) {
	setAuditViewer(request, profile)
	if credential.Authenticator.CloneWarning && auth.audit != nil {
		auth.audit.passkeyRisk(request, profile, credential)
	}
}

func (auth *passkeyAuth) passkeyOnboardingNext(profile viewerProfile) bool {
	return profile.Owner && auth.settings != nil && auth.settings.onboardingPending()
}

func subtitlePasskeyIdentity(profile viewerProfile) (string, string, []webauthn.Credential) {
	return profile.ID, profile.Name, profile.Passkeys
}

func subtitlePasskeyIsOwner(profile viewerProfile) bool  { return profile.Owner }
func subtitlePasskeyIsRemote(profile viewerProfile) bool { return profile.Remote }

func (auth *passkeyAuth) revokePublic() {
	if auth == nil {
		return
	}
	auth.engine.RevokePublic()
}
