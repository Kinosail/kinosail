package server

import (
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/viewing"
)

const viewingImportPreviewHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="#0b0d0b"><title>Viewing activity preview · Kinosail Player</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"></head><body class="settings-page"><a class="skip" href="#main">Skip to content</a><main id="main" class="settings-shell"><a class="back" href="{{.BackPath}}">{{icon "back"}} {{.BackLabel}}</a><header class="settings-intro"><span class="eyebrow">{{.Step}}</span><h1>{{.Source}} → {{.ProfileName}}</h1><p>No changes have been made. This preview expires at {{.ExpiresAt.Local.Format "3:04 PM"}}.</p></header><section class="wide"><h2>Summary</h2><p>Watched or in progress: {{.Summary.Activity}} · Jellyfin favorites to add to My List: {{.Summary.Favorites}} · Playlist entries: {{.Summary.PlaylistItems}} · {{.Summary.Importable}} ready · {{.Summary.Unchanged}} unchanged · Existing Kinosail activity kept: {{.Summary.Conflicts}} · {{.Summary.Ambiguous}} ambiguous · {{.Summary.Unmatched}} unmatched</p><div class="profile-row">{{if .Summary.Importable}}<form action="{{.ApplyPath}}" method="post"><input type="hidden" name="id" value="{{.ID}}"><button>Import once</button></form>{{end}}{{if .AllowSync}}<form action="/settings/viewing-syncs" method="post"><input type="hidden" name="id" value="{{.ID}}"><label>Sync automatically <select name="interval"><option value="15m">Every 15 minutes</option><option value="1h" selected>Hourly</option><option value="6h">Every 6 hours</option><option value="24h">Daily</option></select></label><button>Import and sync automatically</button></form>{{end}}</div></section><section class="wide"><h2>Items</h2>{{range .Items}}<p><strong>{{.SourceTitle}}</strong> · {{.Status}} · {{.Reason}}{{if .TargetTitle}} · {{.TargetTitle}}{{end}}{{if .Watched}} · watched{{else if .Seconds}} · resume at {{printf "%.0f" .Seconds}}s{{end}}{{if .Favorite}} · add to My List{{end}}{{if .Playlists}} · playlists: {{range $index, $name := .Playlists}}{{if $index}}, {{end}}{{$name}}{{end}}{{end}}</p>{{else}}<p>No watched, in-progress, favorite, or playlist items were returned by the source.</p>{{end}}{{if .Truncated}}<p>Only the first 500 preview rows are shown; all matched items will still be processed.</p>{{end}}</section></main></body></html>`

const viewingImportResultHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="#0b0d0b"><title>Viewing activity imported · Kinosail Player</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"></head><body class="settings-page"><main class="settings-shell"><a class="back" href="/settings#viewing-imports">{{icon "back"}} Settings</a><header class="settings-intro"><span class="eyebrow">Migration complete</span><h1>Viewing activity imported</h1><p>Viewing activity updated: {{.Applied}} · Favorites or playlist entries added: {{.ListsApplied}} · {{.Unchanged}} unchanged · Existing activity kept: {{.Conflicts}} · {{.Ambiguous}} ambiguous · {{.Unmatched}} unmatched.</p></header><p><a class="mode" href="/">Open Library</a></p></main></body></html>`

const viewingImportOnboardingHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="#0b0d0b"><title>Move to Kinosail Player</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"></head><body class="settings-page onboarding-page"><a class="skip" href="#main">Skip to content</a><main id="main" class="settings-shell onboarding-shell">{{template "onboardingProgress"}}<header class="settings-intro"><span class="eyebrow">Step 4 of 4 · Viewing history</span><h1>Import your viewing history.</h1><p>Connect Plex or Jellyfin to preview watched state, resume positions, Jellyfin favorites, and video playlists before Kinosail changes anything. Because My List is Kinosail's private favorite set, matched Jellyfin favorites become My List entries.</p><p>One-time import · credentials are discarded when the preview expires</p></header><section class="wide"><h2>Choose one source profile</h2><p>Viewing activity belongs to people, so Kinosail imports into one Viewer Profile at a time. Repeat this step for anyone else. Passwords, PINs, permissions, and source credentials do not transfer.</p><p>Plex Universal Watchlist is hosted account data and is not exposed by Plex's documented local Server interface, so this step imports Plex Server playlists but not Universal Watchlist entries.</p><form action="/onboarding/viewing-imports/preview" method="post"><label>Source <select name="source"><option value="plex">Plex</option><option value="jellyfin">Jellyfin</option></select></label><label>Server URL <input name="url" type="url" placeholder="http://media-server:32400" required></label><label>Access token <input name="token" type="password" autocomplete="off" required></label><label>Source user ID <input name="sourceUser" placeholder="Optional for a Jellyfin user token"></label><label>Destination Viewer Profile <select name="profileId">{{range .}}<option value="{{.ID}}">{{.Name}}</option>{{end}}</select></label>{{template "viewingOverwriteField"}}<button>Preview import</button></form></section><p><a class="mode" href="/onboarding/finish">Finish and open Library</a> <a class="mode" href="/settings#profiles">Create or manage Viewer Profiles</a></p></main></body></html>`

const viewingImportControlHTML = `{{define "viewingOverwriteField"}}<label><input type="checkbox" name="overwriteExisting" value="true"> Replace existing Kinosail watched and resume state (off preserves it)</label>{{end}}`

var (
	viewingImportPreviewView    = newLocalizedTemplate("viewing-import-preview", viewingImportPreviewHTML)
	viewingImportResultView     = newLocalizedTemplate("viewing-import-result", viewingImportResultHTML)
	viewingImportOnboardingView = newLocalizedTemplate("viewing-import-onboarding", ignoreNonPasswordSecretAutofill(viewingImportControlHTML+`{{define "onboardingProgress"}}`+onboardingProgressHTML+`{{end}}`+updateChoicePage(strings.Replace(strings.Replace(viewingImportOnboardingHTML, `{{template "onboardingProgress"}}`, `{{template "onboardingProgress" .Step}}`, 1), `{{range .}}`, `{{range .Profiles}}`, 1))))
)

type viewingImportWebPage struct {
	viewingImportPreview
	BackPath, BackLabel, Step, ApplyPath string
	AllowSync                            bool
}

func (manager *viewingImportManager) register(mux *http.ServeMux, auth *authentication) {
	viewing.RegisterHTTP(mux, auth.owner, manager)
}

func (manager *viewingImportManager) WebOnboarding(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if err := viewingImportOnboardingView.Execute(writer, request, struct {
		Profiles []apiProfile
		Step     string
	}{publicProfiles(manager.profiles.list()), "arrival"}); err != nil {
		localizedError(writer, request, "viewing activity onboarding failed", http.StatusInternalServerError)
	}
}

func (manager *viewingImportManager) APIPreview(writer http.ResponseWriter, request *http.Request) {
	var input viewingImportInput
	if !readJSON(writer, request, &input) {
		return
	}
	preview, err := manager.previewer.Preview(request.Context(), input)
	if err != nil {
		apiError(writer, err, http.StatusBadRequest)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, preview, http.StatusOK)
}

func (manager *viewingImportManager) APIApply(writer http.ResponseWriter, request *http.Request) { //nolint:contextcheck // A confirmed import commits every category despite client cancellation.
	result, err := manager.previewer.Apply(request.PathValue("id")) //nolint:contextcheck // A confirmed import durably commits every category.
	if err != nil {
		apiError(writer, err, http.StatusConflict)
		return
	}
	writeJSON(writer, result, http.StatusOK)
}

func (manager *viewingImportManager) APIListSyncs(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, map[string]any{"syncs": manager.list()}, http.StatusOK)
}

func (manager *viewingImportManager) APICreateSync(writer http.ResponseWriter, request *http.Request) { //nolint:contextcheck // Sync creation and its initial import share a durable finalization boundary.
	var input struct {
		PreviewID string `json:"previewId"`
		Interval  string `json:"interval"`
	}
	if !readJSON(writer, request, &input) {
		return
	}
	view, err := manager.createSync(input.PreviewID, input.Interval) //nolint:contextcheck // Sync creation and its initial import finalize together.
	if err != nil {
		apiError(writer, err, http.StatusBadRequest)
		return
	}
	writeJSON(writer, view, http.StatusCreated)
}

func (manager *viewingImportManager) APIRunSync(writer http.ResponseWriter, request *http.Request) {
	view, err := manager.run(request.Context(), request.PathValue("id"))
	if err != nil {
		apiError(writer, err, http.StatusBadGateway)
		return
	}
	writeJSON(writer, view, http.StatusOK)
}

func (manager *viewingImportManager) APIDeleteSync(writer http.ResponseWriter, request *http.Request) {
	if err := manager.remove(request.PathValue("id")); err != nil {
		apiError(writer, err, http.StatusNotFound)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (manager *viewingImportManager) WebPreview(writer http.ResponseWriter, request *http.Request) {
	status, err := viewing.ServeWebPreview(request.Context(), writer, request, manager.previewer.Preview, func(page viewing.WebPreview) error {
		return viewingImportPreviewView.Execute(writer, request, viewingImportWebPage{page.Preview, page.BackPath, page.BackLabel, page.Step, page.ApplyPath, page.AllowSync})
	})
	if err != nil {
		localizedError(writer, request, err.Error(), status)
	}
}

func (manager *viewingImportManager) WebApply(writer http.ResponseWriter, request *http.Request) { //nolint:contextcheck // A confirmed import commits every category despite client cancellation.
	result, status, err := viewing.ApplyForm(request, manager.previewer.Apply) //nolint:contextcheck // A confirmed import durably commits every category.
	if err != nil {
		localizedError(writer, request, err.Error(), status)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if err := viewingImportResultView.Execute(writer, request, result); err != nil {
		localizedError(writer, request, "viewing activity result failed", http.StatusInternalServerError)
	}
}

func (manager *viewingImportManager) WebCreateSync(writer http.ResponseWriter, request *http.Request) { //nolint:contextcheck // Sync creation and its initial import share a durable finalization boundary.
	id, interval, err := viewingSyncForm(request)
	if err != nil {
		localizedError(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := manager.createSync(id, interval); err != nil { //nolint:contextcheck // Sync creation and its initial import finalize together.
		localizedError(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(writer, request, "/settings#viewing-imports", http.StatusSeeOther)
}

func (manager *viewingImportManager) WebRunSync(writer http.ResponseWriter, request *http.Request) {
	id, err := viewingActionForm(request)
	if err != nil {
		localizedError(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := manager.run(request.Context(), id); err != nil {
		localizedError(writer, request, err.Error(), http.StatusBadGateway)
		return
	}
	http.Redirect(writer, request, "/settings#viewing-imports", http.StatusSeeOther)
}

func (manager *viewingImportManager) WebDeleteSync(writer http.ResponseWriter, request *http.Request) {
	id, err := viewingActionForm(request)
	if err != nil {
		localizedError(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	if err := manager.remove(id); err != nil {
		localizedError(writer, request, err.Error(), http.StatusNotFound)
		return
	}
	http.Redirect(writer, request, "/settings#viewing-imports", http.StatusSeeOther)
}

func viewingActionForm(request *http.Request) (string, error) {
	return viewing.ParseActionForm(request)
}

func viewingSyncForm(request *http.Request) (string, string, error) {
	return viewing.ParseSyncForm(request)
}
