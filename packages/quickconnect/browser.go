package quickconnect

import (
	"errors"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/apihttp"
	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/identitycore"
)

const browserCookieName = "__Host-kinosail_quick_connect"

// StartBrowser keeps the one-time secret in an HttpOnly cookie, never page or URL data.
func (application *Application) StartBrowser(writer http.ResponseWriter, request *http.Request) {
	if !browserRequest(writer, request) {
		return
	}
	previous, valid := browserSecret(request)
	if !valid {
		apihttp.Error(writer, errHTTPRequest, http.StatusBadRequest)
		return
	}
	if !application.limiters.Starts.Allow(httpguard.RemoteHost(request.RemoteAddr), 20) {
		tooMany(writer, "too many Quick Connect requests")
		return
	}
	secret, pending, err := application.broker.Create(Request{
		Device: application.adapter.CleanDevice(request.UserAgent()), Client: "Kinosail browser", Version: "1", Numeric: true, Remote: true,
	})
	if err != nil {
		apihttp.Error(writer, errHTTPRequest, approvalStatus(err))
		return
	}
	if previous != "" {
		_ = application.broker.Cancel(previous)
	}
	http.SetCookie(writer, browserCookie(secret, int(application.broker.ttl.Seconds())))
	apihttp.WriteJSON(writer, map[string]any{"code": pending.Code, "expiresIn": int(application.broker.ttl.Seconds())}, http.StatusCreated)
}

// PollBrowser issues a public browser session only after local strong Viewer approval.
func (application *Application) PollBrowser(writer http.ResponseWriter, request *http.Request) {
	if !browserRequest(writer, request) {
		return
	}
	secret, valid := browserSecret(request)
	if !valid || secret == "" {
		apihttp.NotFound(writer)
		return
	}
	if !application.limiters.Polls.Allow(httpguard.RemoteHost(request.RemoteAddr), 120) {
		tooMany(writer, "too many Quick Connect polls")
		return
	}
	grant, err := application.broker.Consume(secret)
	if errors.Is(err, ErrPending) {
		apihttp.WriteJSON(writer, map[string]string{"status": "pending"}, http.StatusAccepted)
		return
	}
	if err != nil || !grant.Remote {
		apihttp.NotFound(writer)
		return
	}
	profile, found := application.adapter.Profiles.Find(grant.ProfileID)
	if !found || profile.Revision != grant.ProfileRevision || application.adapter.SignInPublic == nil {
		apihttp.NotFound(writer)
		return
	}
	if err := application.adapter.SignInPublic(writer, request, profile.ID, grant.ProfileRevision); err != nil {
		apihttp.Error(writer, errors.New("could not sign in; request a new code"), http.StatusConflict)
		return
	}
	http.SetCookie(writer, browserCookie("", -1))
	writer.WriteHeader(http.StatusNoContent)
}

// CancelBrowser withdraws this browser's request without accepting a caller-supplied secret.
func (application *Application) CancelBrowser(writer http.ResponseWriter, request *http.Request) {
	if !browserRequest(writer, request) {
		return
	}
	secret, valid := browserSecret(request)
	if !valid {
		apihttp.Error(writer, errHTTPRequest, http.StatusBadRequest)
		return
	}
	if secret != "" {
		_ = application.broker.Cancel(secret)
	}
	http.SetCookie(writer, browserCookie("", -1))
	writer.WriteHeader(http.StatusNoContent)
}

func browserRequest(writer http.ResponseWriter, request *http.Request) bool {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	if !identitycore.RemoteRequest(request) {
		apihttp.NotFound(writer)
		return false
	}
	origins := request.Header.Values("Origin")
	site := request.Header.Get("Sec-Fetch-Site")
	if len(origins) != 1 || origins[0] != "https://"+request.Host || site != "" && site != "same-origin" {
		apihttp.Error(writer, errors.New("open the public sign-in page and try again"), http.StatusForbidden)
		return false
	}
	if request.Method != http.MethodPost || request.URL.ForceQuery || request.Header.Get("Content-Type") != "" || !httpguard.EmptyMutationRequest(writer, request) {
		apihttp.Error(writer, errHTTPRequest, http.StatusBadRequest)
		return false
	}
	return true
}

func browserSecret(request *http.Request) (string, bool) {
	secret, count := "", 0
	for _, cookie := range request.Cookies() {
		if cookie.Name == browserCookieName {
			secret, count = cookie.Value, count+1
		}
	}
	return secret, count <= 1 && (count == 0 || identitycore.ValidSessionToken(secret) && len(secret) <= 128)
}

func browserCookie(secret string, age int) *http.Cookie {
	return &http.Cookie{Name: browserCookieName, Value: secret, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: age, Expires: time.Now().Add(time.Duration(age) * time.Second)}
}
