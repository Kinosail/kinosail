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

func (store *settingsStore) navigationController() sharednavigation.Controller {
	return sharednavigation.NewController(store.navigation, store.setNavigation, func(language, name string) string {
		return localeCatalog.Localize(language, name)
	})
}

var applicationNavigationView = newLocalizedTemplate("application-navigation", subtitleAppHeader)

type applicationNavigationData struct {
	ServerName                        string
	Owner                             bool
	ViewerName                        string
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
		if matched == "" && request.Method == http.MethodGet && !strings.HasPrefix(request.URL.Path, "/api/") && !strings.HasPrefix(request.URL.Path, "/static/") {
			localizedPageError(writer, request, "Page not found", "That Subtitles page does not exist. Check the address or return to Subtitles.", http.StatusNotFound)
			return
		}
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
		data := applicationNavigationData{ServerName: settings.serverName(), Owner: viewer.Owner, ViewerName: viewer.Name, CanLogout: viewer.ID != "local-owner", View: applicationShellView(matched), Sort: "title", UpdateAvailable: updates != nil && updates.Available()}
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
	case "GET /account", "GET /settings", "GET /settings/agent-connections", "GET /settings/backups", "GET /settings/configuration", "GET /settings/management", "GET /settings/system", "GET /supporter":
		return true
	}
	return false
}

func applicationShellView(pattern string) string {
	if strings.HasPrefix(pattern, "GET /settings") {
		return "settings"
	}
	return ""
}

type pageEdit struct {
	at         int
	text       string
	navigation []byte
}

func injectApplicationShell(page, navigation []byte) []byte {
	bodyStart := bytes.Index(page, []byte("<body"))
	if bodyStart < 0 {
		return page
	}
	bodyEnd := bodyStart + bytes.IndexByte(page[bodyStart:], '>')
	if bodyEnd < bodyStart {
		return page
	}
	edits := make([]pageEdit, 0, 3)
	if class := bytes.Index(page[bodyStart:bodyEnd], []byte(`class="`)); class >= 0 {
		edits = append(edits, pageEdit{at: bodyStart + class + len(`class="`), text: "library-page subtitle-app subtitle-dashboard "})
	} else {
		edits = append(edits, pageEdit{at: bodyEnd, text: ` class="library-page subtitle-app subtitle-dashboard"`})
	}
	if mainStart := bytes.Index(page, []byte("<main")); mainStart >= 0 {
		if mainEnd := mainStart + bytes.IndexByte(page[mainStart:], '>'); mainEnd >= mainStart && !bytes.Contains(page[mainStart:mainEnd], []byte(`id="`)) {
			edits = append(edits, pageEdit{at: mainStart + len("<main"), text: ` id="main"`})
		}
	}
	insertAt, prefix := bodyEnd+1, `<a class="skip" href="#main">Skip to content</a>`
	if skip := bytes.Index(page[insertAt:], []byte(`<a class="skip"`)); skip >= 0 {
		skip += insertAt
		if end := bytes.Index(page[skip:], []byte("</a>")); end >= 0 {
			insertAt, prefix = skip+end+len("</a>"), ""
		}
	}
	return applyPageEdits(page, append(edits, pageEdit{at: insertAt, text: "\n" + prefix + "\n", navigation: navigation}))
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
