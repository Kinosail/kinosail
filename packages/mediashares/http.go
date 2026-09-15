package mediashares

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

const (
	claimViewSource = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="#090a08"><title>Open Media Share · __PRODUCT__</title><script src="/static/theme.js?v=__ASSET__"></script><link rel="stylesheet" href="/static/app.css?v=__ASSET__"></head><body class="auth"><main class="grant-card"><span class="eyebrow">Private Media Share</span><h1>Opening shared media</h1><p id="status">Checking this private link…</p><script src="/static/media-share.js"></script></main></body></html>`
	itemsViewSource = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="#090a08"><title>Media Share · __PRODUCT__</title><script src="/static/theme.js?v=__ASSET__"></script><link rel="stylesheet" href="/static/app.css?v=__ASSET__"></head><body class="player-page"><main class="player-shell"><a class="back" href="/">Kinosail</a><header class="title-block"><span class="eyebrow">Private Media Share</span><h1>Shared with you</h1></header>{{range .}}<article class="shared-media"><h2>{{.Title}}</h2><div class="media-stage"><video controls preload="metadata" src="/share/media/{{.ID}}"></video></div></article>{{end}}</main></body></html>`
	ownerViewSource = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="#090a08"><title>Media Shares · __PRODUCT__</title><script src="/static/theme.js?v=__ASSET__"></script><link rel="stylesheet" href="/static/app.css?v=__ASSET__"></head><body class="settings-page"><main class="settings-shell"><a class="back" href="/settings">{{icon "back"}} Settings</a><header class="settings-intro"><span class="eyebrow">Private access</span><h1>Media Shares</h1><p>Create an expiring link for selected Library Content and a limited number of devices.</p></header>{{if .ClaimURL}}<p role="status">Copy this link now: <code>{{.ClaimURL}}</code></p>{{end}}<div class="settings-flow"><section class="wide"><h2>Create a Media Share</h2><form class="media-share-form" method="post" action="/settings/media-shares"><fieldset><legend>Library Content</legend>{{range .Items}}<label><input type="checkbox" name="itemIds" value="{{.ID}}"> {{.Title}}</label>{{end}}</fieldset><div class="profile-row"><fieldset class="choice-set segmented-choice"><legend>Expires</legend><label class="choice-option"><input type="radio" name="expires" value="3600" checked><span>One hour</span></label><label class="choice-option"><input type="radio" name="expires" value="86400"><span>24 hours</span></label></fieldset><fieldset class="choice-set segmented-choice"><legend>Devices</legend><label class="choice-option"><input type="radio" name="devices" value="1" checked><span>One</span></label><label class="choice-option"><input type="radio" name="devices" value="2"><span>Two</span></label><label class="choice-option"><input type="radio" name="devices" value="4"><span>Four</span></label></fieldset></div><label><input type="checkbox" name="rightsAcknowledged" value="true" required> I confirm I have the right to share these items with recipients.</label><button>Create secure Media Share</button></form></section><section class="wide"><h2>Active</h2>{{range .Shares}}<form method="post" action="/settings/media-shares/{{.ID}}/revoke"><span>Expires {{.ExpiresAt}} · {{len .ItemIDs}} items</span><button class="danger">Revoke</button></form>{{else}}<p>No active Media Shares.</p>{{end}}</section></div></main></body></html>`
)

var claimScript = []byte(`(()=>{const status=document.getElementById("status"),token=location.hash.slice(1),csrf=document.querySelector('meta[name="kinosail-csrf"]')?.content;history.replaceState(null,"",location.pathname);if(!token){status.textContent="This Media Share link is incomplete.";return}const headers={"Content-Type":"application/json"};if(csrf)headers["X-Kinosail-CSRF"]=csrf;fetch("/api/v1/media-shares/claim",{method:"POST",headers,body:JSON.stringify({token,device:navigator.userAgent.slice(0,80)})}).then(r=>{if(!r.ok)throw 0;location.replace("/share/items")}).catch(()=>status.textContent="This Media Share is unavailable or expired.")})()`)

// Views contains the Player-derived media-share templates for one application brand.
type Views struct {
	Claim *template.Template
	Items *template.Template
	Owner *template.Template
}

// NewViews derives one application's media-share pages from Player's canonical markup.
func NewViews(product, assetVersion string, csrfTemplate func(string, string) *template.Template) Views {
	if !validViewConfiguration(product, assetVersion, csrfTemplate != nil) {
		panic("invalid media share view configuration")
	}
	adapt := func(source string) string {
		return strings.NewReplacer("__PRODUCT__", "Kinosail "+product, "__ASSET__", assetVersion).Replace(source)
	}
	return Views{
		Claim: csrfTemplate("media-share", adapt(claimViewSource)),
		Items: template.Must(template.New("media-share-items").Parse(adapt(itemsViewSource))),
		Owner: csrfTemplate("media-share-owner", adapt(ownerViewSource)),
	}
}

func validViewConfiguration(product, assetVersion string, hasCSRFTemplate bool) bool {
	return hasCSRFTemplate && safeViewToken(product, 32) && safeAssetVersion(assetVersion)
}

type (
	executeCSRF    func(*template.Template, http.ResponseWriter, *http.Request, any) error
	localizedError func(http.ResponseWriter, *http.Request, string, int)
)

type web struct {
	store     *Store
	views     Views
	execute   executeCSRF
	localized localizedError
}

// Register installs the public and owner web routes around a shared Store.
func Register(mux *http.ServeMux, store *Store, views Views, owner func(http.Handler) http.Handler, script func([]byte) http.HandlerFunc, execute executeCSRF, localized localizedError) {
	if !validHTTPDependencies(mux != nil, store != nil, owner != nil, script != nil, execute != nil, localized != nil) {
		panic("invalid media share HTTP dependencies")
	}
	handlers := web{store: store, views: views, execute: execute, localized: localized}
	mux.HandleFunc("GET /share", handlers.claimPage)
	mux.HandleFunc("GET /static/media-share.js", script(append([]byte(nil), claimScript...)))
	mux.HandleFunc("POST /api/v1/media-shares/claim", store.ClaimHTTP)
	mux.HandleFunc("GET /share/items", handlers.items)
	mux.HandleFunc("GET /share/media/{id}", store.Serve)
	mux.HandleFunc("HEAD /share/media/{id}", store.Serve)
	mux.Handle("GET /settings/media-shares", owner(http.HandlerFunc(handlers.ownerPage)))
	mux.Handle("POST /settings/media-shares", owner(http.HandlerFunc(handlers.create)))
	mux.Handle("POST /settings/media-shares/{id}/revoke", owner(http.HandlerFunc(handlers.revoke)))
}

func validHTTPDependencies(present ...bool) bool {
	return len(present) == 6 && slices.Index(present, false) == -1
}

func (handlers web) claimPage(writer http.ResponseWriter, request *http.Request) {
	_ = handlers.execute(handlers.views.Claim, writer, request, nil)
}

func (handlers web) items(writer http.ResponseWriter, request *http.Request) {
	items, ok := handlers.store.Items(request)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	_ = handlers.views.Items.Execute(writer, items)
}

func (handlers web) ownerPage(writer http.ResponseWriter, request *http.Request) {
	handlers.renderOwnerPage(writer, request, "")
}

func (handlers web) renderOwnerPage(writer http.ResponseWriter, request *http.Request, claimURL string) {
	shares, err := handlers.store.List()
	if err != nil {
		http.Error(writer, "media shares are unavailable", http.StatusServiceUnavailable)
		return
	}
	items, err := handlers.store.snapshot()
	if err != nil {
		http.Error(writer, "media shares are unavailable", http.StatusServiceUnavailable)
		return
	}
	_ = handlers.execute(handlers.views.Owner, writer, request, struct {
		Items    any
		Shares   []View
		ClaimURL string
	}{items, shares, claimURL})
}

func (handlers web) create(writer http.ResponseWriter, request *http.Request) {
	lifetime, devices, acknowledged, err := decodeCreateForm(writer, request)
	if err != nil {
		handlers.localized(writer, request, "media share policy is invalid", http.StatusBadRequest)
		return
	}
	_, token, err := handlers.store.Create(request.PostForm["itemIds"], lifetime, devices, acknowledged)
	if err != nil {
		handlers.localized(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	handlers.renderOwnerPage(writer, request, "/share#"+token)
}

func (handlers web) revoke(writer http.ResponseWriter, request *http.Request) {
	if err := handlers.store.Revoke(request.PathValue("id")); err != nil {
		handlers.localized(writer, request, "not found", http.StatusNotFound)
		return
	}
	http.Redirect(writer, request, "/settings/media-shares", http.StatusSeeOther)
}

// CreateHTTP validates and creates an owner API grant.
func (store *Store) CreateHTTP(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		ItemIDs            []string `json:"itemIds"`
		ExpiresInSeconds   int64    `json:"expiresInSeconds"`
		MaxDevices         int      `json:"maxDevices"`
		RightsAcknowledged bool     `json:"rightsAcknowledged"`
	}
	if err := httpguard.DecodeRequestJSON(writer, request, &input); err != nil {
		writeError(writer, err, http.StatusBadRequest)
		return
	}
	if !validLifetimeSeconds(input.ExpiresInSeconds) {
		writeError(writer, errors.New("media share policy is invalid"), http.StatusBadRequest)
		return
	}
	share, token, err := store.Create(input.ItemIDs, time.Duration(input.ExpiresInSeconds)*time.Second, input.MaxDevices, input.RightsAcknowledged)
	if err != nil {
		writeError(writer, err, http.StatusBadRequest)
		return
	}
	writeJSON(writer, map[string]any{"id": share.ID, "itemIds": share.ItemIDs, "expiresAt": share.ExpiresAt, "maxDevices": share.MaxDevices, "claimToken": token, "claimURL": "/share#" + token}, http.StatusCreated)
}

// ListHTTP returns active grants to an owner.
func (store *Store) ListHTTP(writer http.ResponseWriter, _ *http.Request) {
	shares, err := store.List()
	if err != nil {
		writeError(writer, errors.New("media shares are unavailable"), http.StatusServiceUnavailable)
		return
	}
	writeJSON(writer, map[string]any{"shares": shares}, http.StatusOK)
}

// RevokeHTTP removes one owner grant.
func (store *Store) RevokeHTTP(writer http.ResponseWriter, request *http.Request) {
	if err := store.Revoke(request.PathValue("id")); err != nil {
		writeError(writer, errors.New("not found"), http.StatusNotFound)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// ClaimHTTP exchanges a public claim token for a secure cookie.
func (store *Store) ClaimHTTP(writer http.ResponseWriter, request *http.Request) {
	var input struct{ Token, Device string }
	if err := httpguard.DecodeRequestJSON(writer, request, &input); err != nil {
		writeError(writer, err, http.StatusBadRequest)
		return
	}
	token, expires, err := store.Claim(input.Token, input.Device)
	if err != nil {
		status := http.StatusNotFound
		if errors.Is(err, ErrDeviceLimit) {
			status = http.StatusTooManyRequests
		}
		writeError(writer, errors.New("media share is unavailable"), status)
		return
	}
	http.SetCookie(writer, &http.Cookie{Name: "__Host-kinosail_share", Value: token, Path: "/", Expires: expires, MaxAge: int(expires.Sub(store.now()).Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	writer.WriteHeader(http.StatusNoContent)
}

func decodeCreateForm(writer http.ResponseWriter, request *http.Request) (time.Duration, int, bool, error) {
	if !validFormEnvelope(request.ContentLength, request.URL.RawQuery) {
		return 0, 0, false, errors.New("invalid media share form")
	}
	if request.PostForm == nil {
		request.Body = http.MaxBytesReader(writer, request.Body, 64<<10)
		if request.ParseForm() != nil {
			return 0, 0, false, errors.New("invalid media share form")
		}
	}
	if !validCreateForm(request.PostForm) {
		return 0, 0, false, errors.New("invalid media share form")
	}
	expires, expiresErr := strconv.ParseInt(request.PostForm.Get("expires"), 10, 64)
	devices, devicesErr := strconv.Atoi(request.PostForm.Get("devices"))
	if expiresErr != nil || devicesErr != nil || !validLifetimeSeconds(expires) {
		return 0, 0, false, errors.New("invalid media share form")
	}
	return time.Duration(expires) * time.Second, devices, request.PostForm.Get("rightsAcknowledged") == "true", nil
}

func validFormEnvelope(contentLength int64, rawQuery string) bool {
	return contentLength <= 64<<10 && rawQuery == ""
}

func validLifetimeSeconds(value int64) bool {
	return value >= int64(time.Minute/time.Second) && value <= int64((24*time.Hour)/time.Second)
}

func validCreateForm(form map[string][]string) bool {
	if len(form["itemIds"]) == 0 || len(form["itemIds"]) > 100 || len(form["expires"]) != 1 || len(form["devices"]) != 1 || len(form["rightsAcknowledged"]) != 1 {
		return false
	}
	for key, values := range form {
		if !validCreateField(key, values) {
			return false
		}
	}
	return true
}

func validCreateField(key string, values []string) bool {
	if key != "itemIds" && key != "expires" && key != "devices" && key != "rightsAcknowledged" && key != "_csrf" || len(values) == 0 {
		return false
	}
	for _, value := range values {
		if len(value) > 256 {
			return false
		}
	}
	return true
}

func safeViewToken(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character != ' ' && (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}

func safeAssetVersion(value string) bool {
	if value == "" || len(value) > 8 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func writeError(writer http.ResponseWriter, err error, status int) {
	writeJSON(writer, map[string]string{"error": err.Error()}, status)
}

func writeJSON(writer http.ResponseWriter, value any, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
