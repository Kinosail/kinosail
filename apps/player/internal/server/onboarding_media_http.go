package server

import (
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

const (
	onboardingFirstStep = "/onboarding/media"
	maximumMediaQuery   = 4096
)

var mediaShortcutPatterns = []string{
	"Movies", "Films", "TV", "TV Shows", "Shows", "Videos", "Music",
	"srv/media", "home/*/Videos", "home/*/Movies", "Users/*/Movies", "Users/*/Videos", "Users/Public/Videos",
	"Volumes/*", "media/*/*", "mnt/*",
}

const onboardingMediaHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><meta name="theme-color" content="#0b0d0b"><title>Add your movies and shows · Kinosail Player</title><link rel="stylesheet" href="/static/app.css"><script src="/static/theme.js"></script></head><body class="settings-page onboarding-page"><a class="skip" href="#main">Skip to content</a><main id="main" class="settings-shell onboarding-shell">{{template "onboardingProgress" "media"}}<header class="settings-intro"><span class="eyebrow">Step 2 of 5 · Your media</span><h1>Where are your movies and shows?</h1><p>Pick the folders that hold your videos, music, or photos. Kinosail only reads them. It never moves, renames, or deletes your files.</p></header>{{if .Message}}<p class="media-picker-message" role="alert">{{.Message}}</p>{{end}}<div class="settings-flow">
<section class="wide media-picker-chosen" aria-labelledby="media-chosen-title"><h2 id="media-chosen-title">{{if .Libraries}}Your folders{{else}}No folders yet{{end}}</h2>{{if .Libraries}}<ul class="media-picker-list">{{range .Libraries}}<li><span class="media-picker-folder"><strong>{{.Name}}</strong><small>{{.Display}}</small></span><span class="media-picker-count">{{if .Unreadable}}Kinosail cannot read this folder{{else if eq .Videos 0}}No videos found yet{{else if and .Complete (eq .Videos 1)}}1 video found{{else if .Complete}}{{.Videos}} videos found{{else}}More than {{.Videos}} videos found{{end}}</span><form action="/onboarding/media/remove" method="post"><input type="hidden" name="path" value="{{.Path}}"><button class="quiet" aria-label="Remove {{.Name}}">Remove</button></form></li>{{end}}</ul>{{else}}<p>Choose a folder below. You can add more folders later in Settings.</p>{{end}}</section>
{{if .Shortcuts}}<section class="wide" aria-labelledby="media-suggested-title"><h2 id="media-suggested-title">Suggested folders</h2><ul class="media-picker-list">{{range .Shortcuts}}<li><a class="media-picker-folder" href="/onboarding/media?path={{.Query}}"><strong>{{.Name}}</strong><small>{{.Display}}</small></a><form action="/onboarding/media" method="post"><input type="hidden" name="path" value="{{.Path}}"><button aria-label="Add {{.Name}}">Add</button></form></li>{{end}}</ul></section>{{end}}
<section class="wide" aria-labelledby="media-browse-title"><h2 id="media-browse-title">Browse folders</h2><p class="media-picker-location"><span>Looking in</span> <strong>{{.Browse.Display}}</strong></p><div class="media-picker-browse-actions">{{if .Browse.HasParent}}<a class="mode" href="/onboarding/media?path={{.Browse.ParentQuery}}">Up one folder</a>{{end}}{{if eq .Browse.State "included"}}<span class="media-picker-count">Already in your folders</span>{{else if .Browse.CanAdd}}<form action="/onboarding/media" method="post"><input type="hidden" name="path" value="{{.Browse.Path}}"><button>Add this folder</button></form>{{end}}</div>{{if .Browse.Folders}}<ul class="media-picker-list">{{range .Browse.Folders}}<li><a class="media-picker-folder" href="/onboarding/media?path={{.Query}}"><strong>{{.Name}}</strong></a>{{if eq .State "included"}}<span class="media-picker-count">Already included</span>{{else if eq .State "contains"}}<span class="media-picker-count">Has a folder you added</span>{{else}}<form action="/onboarding/media" method="post"><input type="hidden" name="path" value="{{.Path}}"><button class="quiet" aria-label="Add {{.Name}}">Add</button></form>{{end}}</li>{{end}}</ul>{{if .Browse.Truncated}}<p>Only the first folders are shown. Open a folder to look inside it.</p>{{end}}{{else}}<p>There are no folders here.</p>{{end}}</section>
</div><p class="onboarding-actions"><a class="button" href="/onboarding/connection">{{if .Libraries}}Continue{{else}}Skip for now{{end}}</a></p></main></body></html>`

var onboardingMediaView = newLazyLocalizedTemplate("onboarding-media", func() string {
	return `{{define "onboardingProgress"}}` + onboardingProgressHTML + `{{end}}` + onboardingMediaHTML
})

type mediaFolderView struct {
	Name, Path, Query, Display string
	State                      string
	Videos                     int
	Complete, Unreadable       bool
}

type mediaBrowseView struct {
	Path, Display, ParentQuery string
	State                      string
	HasParent, CanAdd          bool
	Truncated                  bool
	Folders                    []mediaFolderView
}

type onboardingMediaPage struct {
	Message   string
	Libraries []mediaFolderView
	Shortcuts []mediaFolderView
	Browse    mediaBrowseView
}

func showOnboardingMedia(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		requested := request.URL.Query().Get("path")
		page := onboardingMediaPage{Message: mediaPickerMessage(request.URL.Query().Get("message"))}
		folders := settings.libraryFolders()
		configured := settings.libraryNamespaces()
		listing, err := folders.Browse(requested)
		if len(requested) > maximumMediaQuery || err != nil {
			page.Message = "That folder cannot be opened. Choose another folder."
			listing, err = folders.Browse(".")
		}
		if err == nil {
			page.Browse = mediaBrowseView{Path: listing.Path, Display: settings.mediaDisplay(listing.Path), HasParent: listing.Path != ".", ParentQuery: url.QueryEscape(listing.Parent), State: libraryState(configured, listing.Path), Truncated: listing.Truncated}
			page.Browse.CanAdd = listing.Path != "." && page.Browse.State == ""
			for _, name := range listing.Folders {
				child := path.Join(listing.Path, name)
				page.Browse.Folders = append(page.Browse.Folders, mediaFolderView{Name: name, Path: child, Query: url.QueryEscape(child), State: libraryState(configured, child)})
			}
		}
		for _, namespace := range configured {
			estimate, err := folders.EstimateVideos(namespace)
			page.Libraries = append(page.Libraries, mediaFolderView{Name: mediaFolderName(namespace, settings), Path: namespace, Display: settings.mediaDisplay(namespace), Videos: estimate.Count, Complete: estimate.Complete, Unreadable: err != nil})
		}
		for _, namespace := range folders.Shortcuts(mediaShortcutPatterns) {
			if libraryState(configured, namespace) == "" {
				page.Shortcuts = append(page.Shortcuts, mediaFolderView{Name: mediaFolderName(namespace, settings), Path: namespace, Query: url.QueryEscape(namespace), Display: settings.mediaDisplay(namespace)})
			}
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := onboardingMediaView.Execute(writer, request, page); err != nil {
			localizedError(writer, request, "media onboarding failed", http.StatusInternalServerError)
		}
	}
}

func changeOnboardingMedia(settings *settingsStore, index *libraryIndex, change func(string) error) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		folder := request.FormValue("path")
		if len(folder) > maximumMediaQuery || change(folder) != nil {
			http.Redirect(writer, request, "/onboarding/media?message=unavailable", http.StatusSeeOther)
			return
		}
		if err := index.UpdateRoots(request.Context(), settings.roots()); err != nil {
			localizedError(writer, request, "could not update library folders", http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, "/onboarding/media", http.StatusSeeOther)
	}
}

func mediaPickerMessage(code string) string {
	if code == "unavailable" {
		return "Kinosail could not use that folder. It may overlap a folder you already added, or Kinosail may not have permission to read it."
	}
	return ""
}

func (store *settingsStore) libraryNamespaces() []string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return append([]string(nil), store.value.Libraries...)
}

func (store *settingsStore) mediaDisplay(namespace string) string {
	return filepath.Join(store.mediaRoot, filepath.FromSlash(namespace))
}

func mediaFolderName(namespace string, settings *settingsStore) string {
	if namespace == "." {
		return filepath.Base(filepath.Clean(settings.mediaRoot))
	}
	return path.Base(namespace)
}

func libraryState(configured []string, namespace string) string {
	state := ""
	for _, existing := range configured {
		if existing == "." || existing == namespace || strings.HasPrefix(namespace, existing+"/") {
			return "included"
		}
		if namespace == "." || strings.HasPrefix(existing, namespace+"/") {
			state = "contains"
		}
	}
	return state
}

func onboardingStep(next string) bool {
	return next == onboardingFirstStep || next == "/onboarding/connection"
}
