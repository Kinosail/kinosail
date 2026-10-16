package server

import (
	"bytes"
	"net/http"
	"slices"
	"strings"

	sharednavigation "github.com/MikeO7/kinosail/packages/navigation"
)

func (store *settingsStore) navigationLinks(view, language string) (primary, more []navigationLink) {
	return store.navigationController().Links(view, language)
}

func (store *settingsStore) navigationPreferences(language string) []navigationPreference {
	return store.navigationController().Preferences(language)
}

func (store *settingsStore) navigationController() sharednavigation.Controller {
	return sharednavigation.NewController(store.navigation, store.setNavigation, func(language, name string) string {
		return localeCatalog.Localize(language, name)
	})
}

func saveNavigation(settings *settingsStore) http.HandlerFunc {
	return settings.navigationController().Handler(localizedError)
}

var applicationNavigationView = newLocalizedTemplate("application-navigation", appHeader)

type applicationNavigationData struct {
	ServerName                        string
	Owner                             bool
	ViewerName, ViewerID              string
	CanLogout                         bool
	Query                             string
	View                              string
	Sort                              string
	NavigationPrimary, NavigationMore []navigationLink
	UpdateAvailable                   bool
	CommandMenu                       bool
}

func withApplicationShell(settings *settingsStore, updates *updateChecker, pattern func(*http.Request) string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		matched := pattern(request)
		if !applicationShellRoute(matched) {
			next.ServeHTTP(writer, request)
			return
		}
		page := newBufferedPage()
		next.ServeHTTP(page, request)
		if !strings.Contains(page.Header().Get("Content-Type"), "text/html") || page.status < 200 || page.status >= 300 {
			page.writeTo(writer)
			return
		}
		viewer := currentViewer(request)
		data := applicationNavigationData{ServerName: settings.serverName(), Owner: viewer.Owner, ViewerName: viewer.Name, ViewerID: viewer.ID, CanLogout: viewer.ID != "local-owner", View: applicationShellView(matched), Sort: "title", UpdateAvailable: updates != nil && updates.Available()}
		data.NavigationPrimary, data.NavigationMore = settings.navigationLinks(data.View, preferredLanguage(request))
		navigation := newBufferedPage()
		if err := applicationNavigationView.Execute(navigation, request, data); err != nil {
			page.writeTo(writer)
			return
		}
		page.body = *bytes.NewBuffer(injectApplicationShell(page.body.Bytes(), navigation.body.Bytes()))
		page.Header().Del("Content-Length")
		page.writeTo(writer)
	})
}

func applicationShellRoute(pattern string) bool {
	switch pattern {
	case "GET /actor", "GET /account", "GET /album/{id}", "GET /book/{id}", "GET /collection/{name}", "GET /metadata/bulk", "GET /offline-downloads", "GET /playlist/{name}", "GET /quick-connect", "GET /settings", "GET /settings/agent-connections", "GET /settings/backups", "GET /settings/configuration", "GET /settings/media-shares", "GET /settings/remote-readiness", "GET /settings/management", "GET /settings/system", "GET /show/{id}", "GET /supporter":
		return true
	}
	return false
}

func applicationShellView(pattern string) string {
	switch pattern {
	case "GET /album/{id}":
		return "music"
	case "GET /book/{id}":
		return "books"
	case "GET /collection/{name}":
		return "collections"
	case "GET /playlist/{name}":
		return "playlists"
	case "GET /show/{id}":
		return "shows"
	}
	return ""
}

type pageEdit struct {
	at         int
	text       string
	navigation []byte
}

func injectApplicationShell(page, navigation []byte) []byte {
	recognition := recognitionScriptEdit(page)
	bodyStart := bytes.Index(page, []byte("<body"))
	if bodyStart < 0 {
		return applyPageEdits(page, recognition)
	}
	bodyEnd := bodyStart + bytes.IndexByte(page[bodyStart:], '>')
	if bodyEnd < bodyStart {
		return applyPageEdits(page, recognition)
	}
	edits := make([]pageEdit, 0, 4)
	if class := bytes.Index(page[bodyStart:bodyEnd], []byte(`class="`)); class >= 0 {
		edits = append(edits, pageEdit{at: bodyStart + class + len(`class="`), text: "library-page "})
	} else {
		edits = append(edits, pageEdit{at: bodyEnd, text: ` class="library-page"`})
	}
	if mainStart := bytes.Index(page, []byte("<main")); mainStart >= 0 {
		if mainEnd := mainStart + bytes.IndexByte(page[mainStart:], '>'); mainEnd >= mainStart {
			attributes := ""
			if !bytes.Contains(page[mainStart:mainEnd], []byte(` id="`)) {
				attributes += ` id="main"`
			}
			if !bytes.Contains(page[mainStart:mainEnd], []byte(` tabindex=`)) {
				attributes += ` tabindex="-1"`
			}
			edits = append(edits, pageEdit{at: mainStart + len("<main"), text: attributes})
		}
	}
	insertAt, prefix := bodyEnd+1, `<a class="skip" href="#main">Skip to content</a>`
	if skip := bytes.Index(page[insertAt:], []byte(`<a class="skip"`)); skip >= 0 {
		skip += insertAt
		if end := bytes.Index(page[skip:], []byte("</a>")); end >= 0 {
			insertAt, prefix = skip+end+len("</a>"), ""
		}
	}
	return applyPageEdits(page, append(append(edits, pageEdit{at: insertAt, text: "\n" + prefix + "\n", navigation: navigation}), recognition...))
}

func recognitionScriptEdit(page []byte) []pageEdit {
	if bytes.Contains(page, []byte(`/static/supporter.js`)) {
		return nil
	}
	end := bytes.Index(page, []byte("</body>"))
	if end < 0 {
		return nil
	}
	return []pageEdit{{at: end, text: `<script defer src="` + assetURL("supporter.js") + `"></script>`}}
}

func applyPageEdits(page []byte, edits []pageEdit) []byte {
	slices.SortStableFunc(edits, func(first, second pageEdit) int { return first.at - second.at })
	size := len(page)
	for _, edit := range edits {
		size += len(edit.text) + len(edit.navigation)
	}
	result, cursor := make([]byte, 0, size), 0
	for _, edit := range edits {
		result = append(append(append(result, page[cursor:edit.at]...), edit.text...), edit.navigation...)
		cursor = edit.at
	}
	return append(result, page[cursor:]...)
}

type bufferedPage struct {
	header      http.Header
	body        bytes.Buffer
	status      int
	wroteHeader bool
}

func newBufferedPage() *bufferedPage {
	return &bufferedPage{header: make(http.Header), status: http.StatusOK}
}

func (page *bufferedPage) Header() http.Header { return page.header }

func (page *bufferedPage) WriteHeader(status int) {
	if !page.wroteHeader {
		page.status, page.wroteHeader = status, true
	}
}

func (page *bufferedPage) Write(body []byte) (int, error) {
	page.wroteHeader = true
	if page.header.Get("Content-Type") == "" {
		page.header.Set("Content-Type", http.DetectContentType(body))
	}
	return page.body.Write(body)
}

func (page *bufferedPage) writeTo(writer http.ResponseWriter) {
	for name, values := range page.header {
		writer.Header()[name] = values
	}
	writer.WriteHeader(page.status)
	_, _ = writer.Write(page.body.Bytes())
}
