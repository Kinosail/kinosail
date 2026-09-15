package metadata

import (
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/library"
)

const bulkFormMaximum = 1 << 20

// BulkPage contains the stable shared view model and product-specific assets.
type BulkPage struct {
	Product, ThemeScript, Stylesheet string
	Items                            []library.Item
	Error                            string
}

// BulkRender renders the product-localized shared bulk metadata page.
type BulkRender func(http.ResponseWriter, *http.Request, any) error

// BulkFailure renders a product-localized HTTP error.
type BulkFailure func(http.ResponseWriter, *http.Request, string, int)

// BulkMetadataHTML is Player's canonical bulk metadata page.
const BulkMetadataHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><meta name="theme-color" content="#090a08"><title>Bulk metadata · {{.Product}}</title><script src="{{.ThemeScript}}"></script><link rel="stylesheet" href="{{.Stylesheet}}"></head><body class="settings-page"><a class="skip" href="#main">Skip to content</a><main id="main" class="settings-shell"><a class="back" href="/">{{icon "back"}} Library</a><header class="settings-intro"><span class="eyebrow">Owner controls</span><h1>Edit multiple items</h1><p>Apply the same metadata fields to selected Library Content. Blank fields stay unchanged.</p></header>{{if .Error}}<p role="alert">{{.Error}}</p>{{end}}<form method="post" action="/metadata/bulk"><div class="settings-flow"><section class="wide"><h2>Library Content</h2><fieldset><legend>Select items</legend>{{range .Items}}<label><input type="checkbox" name="itemIds" value="{{.ID}}"> {{.Title}}{{if .Year}} <small>{{.Year}}</small>{{end}}</label>{{else}}<p>No Library Content is available.</p>{{end}}</fieldset></section><section><h2>Metadata changes</h2><label>Title <input name="title" maxlength="200"></label><label>Year <input name="year" maxlength="4" inputmode="numeric"></label><label>Rating <input name="rating" maxlength="32"></label><label>Tagline <input name="tagline" maxlength="300"></label><label>Genres <input name="genres" maxlength="500"></label><label>Plot <textarea name="plot" maxlength="5000"></textarea></label><button>Apply metadata</button></section></div></form></main></body></html>` //nolint:lll // Keeping the compact Player template preserves its exact response body.

// PlayerBulkPage returns Player's canonical bulk metadata product profile.
func PlayerBulkPage() BulkPage {
	return BulkPage{Product: "Kinosail Player", ThemeScript: "/static/theme.js?v=cinema-1", Stylesheet: "/static/app.css?v=cinema-1"}
}

// SubtitlesBulkPage returns the derivative's bulk metadata product profile.
func SubtitlesBulkPage() BulkPage {
	return BulkPage{Product: "Kinosail Subtitles", ThemeScript: "/static/theme.js?v=cinema-1", Stylesheet: "/static/app.css?v=cinema-1"}
}

// Register installs the canonical owner-wrapped bulk metadata routes.
func (editor *BulkEditor) Register(mux *http.ServeMux, owner func(http.Handler) http.Handler) {
	mux.Handle("GET /metadata/bulk", owner(http.HandlerFunc(editor.Page)))
	mux.Handle("POST /metadata/bulk", owner(http.HandlerFunc(editor.Edit)))
}

// Page renders the sorted bulk metadata form.
func (editor *BulkEditor) Page(writer http.ResponseWriter, request *http.Request) {
	editor.renderPage(writer, request, "")
}

// Edit validates and applies one bulk metadata form submission.
func (editor *BulkEditor) Edit(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, bulkFormMaximum)
	if !httpguard.FormEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil ||
		!httpguard.OnlyFormKeys(request.PostForm, "itemIds", "title", "year", "rating", "tagline", "genres", "plot") {
		editor.renderPage(writer, request, "metadata request is invalid")
		return
	}
	patch, err := bulkPatchFromForm(request.PostForm)
	if err != nil {
		editor.renderPage(writer, request, "metadata request is invalid")
		return
	}
	if _, err := editor.Apply(request.Context(), request.PostForm["itemIds"], patch); err != nil {
		editor.renderPage(writer, request, err.Error())
		return
	}
	http.Redirect(writer, request, "/", http.StatusSeeOther)
}

func (editor *BulkEditor) renderPage(writer http.ResponseWriter, request *http.Request, message string) {
	items, err := editor.index.Snapshot()
	if err != nil {
		editor.fail(writer, request, "library is unavailable", http.StatusServiceUnavailable)
		return
	}
	sort.SliceStable(items, func(left, right int) bool {
		return strings.ToLower(items[left].Title) < strings.ToLower(items[right].Title)
	})
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := editor.page
	page.Items, page.Error = items, message
	if err := editor.render(writer, request, page); err != nil {
		editor.fail(writer, request, err.Error(), http.StatusInternalServerError)
	}
}

func bulkPatchFromForm(form url.Values) (BulkPatch, error) {
	value := func(name string) (*string, error) {
		values, found := form[name]
		if !found {
			return nil, nil
		}
		if len(values) != 1 {
			return nil, ErrBulkFields
		}
		trimmed := strings.TrimSpace(values[0])
		if trimmed == "" {
			return nil, nil
		}
		return &trimmed, nil
	}
	var patch BulkPatch
	var err error
	if patch.Title, err = value("title"); err != nil {
		return BulkPatch{}, err
	}
	if patch.Year, err = value("year"); err != nil {
		return BulkPatch{}, err
	}
	if patch.Rating, err = value("rating"); err != nil {
		return BulkPatch{}, err
	}
	if patch.Tagline, err = value("tagline"); err != nil {
		return BulkPatch{}, err
	}
	if patch.Genres, err = value("genres"); err != nil {
		return BulkPatch{}, err
	}
	if patch.Plot, err = value("plot"); err != nil {
		return BulkPatch{}, err
	}
	return patch, nil
}
