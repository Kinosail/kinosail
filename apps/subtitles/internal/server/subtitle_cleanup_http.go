package server

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

const subtitleCleanupPreviewHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><meta name="theme-color" content="#0b0d0b"><title>Subtitle cleanup · Kinosail Subtitles</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"></head><body class="settings-page subtitle-settings"><a class="skip" href="#main">Skip to content</a><main id="main" class="settings-shell"><a class="back" href="/settings#cleanup">{{icon "back"}} Subtitle settings</a><header class="settings-intro"><h1>Subtitle cleanup</h1><p>Keep {{.Language}} subtitles and make {{.Language}} your only preferred language. {{if eq .Forced "keep"}}Keep forced {{.Language}} subtitles.{{else}}Delete forced {{.Language}} subtitles.{{end}}</p></header><section class="wide"><h2>{{.Count}} subtitle {{if eq .Count 1}}file{{else}}files{{end}} to delete</h2><p>Only tagged .srt and .vtt files beside videos are included. Embedded tracks and files with uncertain language stay in place. {{.Skipped}} files were skipped.{{if gt .Count (len .Files)}} The first {{len .Files}} matches are shown.{{end}}</p>{{if .Files}}<ul>{{range .Files}}<li><code>{{.Path}}</code></li>{{end}}</ul>{{else}}<p>No files match this cleanup choice.</p>{{end}}<form action="/settings/subtitles/cleanup" method="post"><input type="hidden" name="language" value="{{.Language}}"><input type="hidden" name="forced" value="{{.Forced}}"><input type="hidden" name="digest" value="{{.Digest}}"><button>{{if .Files}}Delete {{.Count}} subtitle {{if eq .Count 1}}file{{else}}files{{end}}{{else}}Keep only {{.Language}}{{end}}</button></form></section></main></body></html>`

var subtitleCleanupPreviewView = newLocalizedTemplate("subtitle-cleanup-preview", subtitleCleanupPreviewHTML)

type subtitleCleanupPreviewData struct {
	Language, Forced, Digest string
	Files                    []subtitleCleanupFile
	Count                    int
	Skipped                  int
}

type subtitleCleanupDoneData struct {
	Language string
	Removed  int
}

func subtitleCleanupInput(request *http.Request, applying bool) (string, string, string, error) {
	values, err := subtitleCleanupValues(request, applying)
	if err != nil {
		return "", "", "", err
	}
	want := 2
	if applying {
		want = 3
	}
	if len(values) != want || len(values["language"]) != 1 || len(values["forced"]) != 1 || applying && len(values["digest"]) != 1 {
		return "", "", "", errors.New("subtitle cleanup request is invalid")
	}
	language, forced := values.Get("language"), values.Get("forced")
	canonical, err := validateSubtitleLanguages([]string{language})
	if err != nil || !oneOf(forced, "keep", "delete") {
		return "", "", "", errors.New("subtitle cleanup request is invalid")
	}
	return canonical[0], forced, values.Get("digest"), nil
}

func subtitleCleanupValues(request *http.Request, applying bool) (url.Values, error) { //nolint:cyclop // Keep strict body, query, and CSRF field validation together at the request boundary.
	if applying {
		request.Body = http.MaxBytesReader(nil, request.Body, 1024)
		if request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.ParseForm() != nil || request.URL.RawQuery != "" {
			return nil, errors.New("subtitle cleanup request is invalid")
		}
	} else if len(request.URL.RawQuery) > 64 || request.URL.RawQuery == "" {
		return nil, errors.New("subtitle cleanup request is invalid")
	}
	values := request.Form
	if !applying {
		var err error
		values, err = url.ParseQuery(request.URL.RawQuery)
		if err != nil {
			return nil, errors.New("subtitle cleanup request is invalid")
		}
	} else if csrf, present := values["_csrf"]; present {
		if len(csrf) != 1 || csrf[0] == "" || len(csrf[0]) > 128 {
			return nil, errors.New("subtitle cleanup request is invalid")
		}
		delete(values, "_csrf")
	}
	return values, nil
}

func previewSubtitleCleanup(index *libraryIndex, settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		language, forced, _, err := subtitleCleanupInput(request, false)
		if err != nil {
			localizedError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		if settings == nil || !slices.Equal(settings.subtitleLanguages(), []string{language}) && settings.editable("subtitles.language") != nil {
			localizedError(writer, request, "preferred subtitle language is managed by deployment configuration", http.StatusConflict)
			return
		}
		plan, err := planSubtitleCleanup(index, language, forced)
		if err != nil {
			localizedError(writer, request, "subtitle cleanup preview is unavailable", http.StatusServiceUnavailable)
			return
		}
		files := plan.Files
		if len(files) > 100 {
			files = files[:100]
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := subtitleCleanupPreviewView.Execute(writer, request, subtitleCleanupPreviewData{Language: language, Forced: forced, Digest: plan.Digest, Files: files, Count: len(plan.Files), Skipped: plan.Skipped}); err != nil {
			localizedError(writer, request, "subtitle cleanup preview is unavailable", http.StatusInternalServerError)
		}
	}
}

func deleteSubtitleCleanup(index *libraryIndex, settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		language, forced, digest, err := subtitleCleanupInput(request, true)
		if err != nil {
			localizedError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		if len(digest) != 64 || strings.Trim(digest, "0123456789abcdef") != "" {
			localizedError(writer, request, "subtitle cleanup preview has expired", http.StatusBadRequest)
			return
		}
		removed, err := applySubtitleCleanup(index, settings, language, forced, digest)
		if err != nil {
			localizedError(writer, request, "subtitle files changed or could not be deleted; preview again", http.StatusConflict)
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := subtitleCleanupDoneView.Execute(writer, request, subtitleCleanupDoneData{Language: language, Removed: removed}); err != nil {
			localizedError(writer, request, "subtitle cleanup result is unavailable", http.StatusInternalServerError)
		}
	}
}

var subtitleCleanupDoneView = newLocalizedTemplate("subtitle-cleanup-done", `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><meta name="theme-color" content="#0b0d0b"><title>Subtitle cleanup · Kinosail Subtitles</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"></head><body class="settings-page subtitle-settings"><a class="skip" href="#main">Skip to content</a><main id="main" class="settings-shell"><a class="back" href="/settings#cleanup">{{icon "back"}} Subtitle settings</a><header class="settings-intro"><h1>Subtitle cleanup complete</h1><p>{{.Language}} is now your only preferred language. Deleted {{.Removed}} subtitle {{if eq .Removed 1}}file{{else}}files{{end}}.</p></header></main></body></html>`)
