package server

import (
	"bytes"
	"net/http"
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

func injectApplicationShell(page, navigation []byte) []byte {
	page = applicationShellCSSVersion(page)
	if !bytes.Contains(page, []byte(`/static/supporter.js?v=`)) {
		page = bytes.Replace(page, []byte("</body>"), []byte(`<script defer src="/static/supporter.js?v=13"></script></body>`), 1)
	}
	bodyStart := bytes.Index(page, []byte("<body"))
	if bodyStart < 0 {
		return page
	}
	bodyEnd := bodyStart + bytes.IndexByte(page[bodyStart:], '>')
	if bodyEnd < bodyStart {
		return page
	}
	body := page[bodyStart:bodyEnd]
	if class := bytes.Index(body, []byte(`class="`)); class >= 0 {
		class += bodyStart + len(`class="`)
		page = append(append(append([]byte(nil), page[:class]...), []byte("library-page ")...), page[class:]...)
		bodyEnd += len("library-page ")
	} else {
		page = append(append(append([]byte(nil), page[:bodyEnd]...), []byte(` class="library-page"`)...), page[bodyEnd:]...)
		bodyEnd += len(` class="library-page"`)
	}
	if mainStart := bytes.Index(page, []byte("<main")); mainStart >= 0 {
		mainEnd := mainStart + bytes.IndexByte(page[mainStart:], '>')
		if mainEnd >= mainStart && !bytes.Contains(page[mainStart:mainEnd], []byte(`id="`)) {
			page = append(append(append([]byte(nil), page[:mainStart+len("<main")]...), []byte(` id="main"`)...), page[mainStart+len("<main"):]...)
		}
	}
	insertAt := bodyEnd + 1
	prefix := []byte(`<a class="skip" href="#main">Skip to content</a>`)
	if skip := bytes.Index(page[insertAt:], []byte(`<a class="skip"`)); skip >= 0 {
		skip += insertAt
		if end := bytes.Index(page[skip:], []byte("</a>")); end >= 0 {
			insertAt = skip + end + len("</a>")
			prefix = nil
		}
	}
	addition := append(append(append([]byte("\n"), prefix...), '\n'), navigation...)
	return append(append(append([]byte(nil), page[:insertAt]...), addition...), page[insertAt:]...)
}

func applicationShellCSSVersion(page []byte) []byte {
	if start := bytes.Index(page, []byte(`/static/app.css?v=`)); start >= 0 {
		version := start + len(`/static/app.css?v=`)
		if end := bytes.IndexByte(page[version:], '"'); end >= 0 {
			if bytes.Equal(page[version:version+end], []byte("electric-20")) {
				return page
			}
			page = append(append(append([]byte(nil), page[:version]...), []byte("electric-20")...), page[version+end:]...)
		}
	}
	return page
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
