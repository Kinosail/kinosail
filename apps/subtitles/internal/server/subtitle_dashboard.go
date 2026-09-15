package server

import (
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type subtitleDashboardItem struct {
	Artwork         bool     `json:"artwork"`
	MediaKind       string   `json:"mediaKind"`
	Year            string   `json:"year,omitempty"`
	State           string   `json:"state"`
	Pending         bool     `json:"pending"`
	Unavailable     bool     `json:"unavailable"`
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Context         string   `json:"context,omitempty"`
	File            string   `json:"file"`
	Tracks          string   `json:"tracks,omitempty"`
	Ready           bool     `json:"ready"`
	Source          string   `json:"source,omitempty"`
	Score           int      `json:"score,omitempty"`
	ReleaseMatch    string   `json:"releaseMatch,omitempty"`
	Installed       string   `json:"installed,omitempty"`
	Cleanup         string   `json:"cleanup,omitempty"`
	Synchronization string   `json:"synchronization,omitempty"`
	LastSearch      string   `json:"lastSearch,omitempty"`
	WantedReason    string   `json:"wantedReason,omitempty"`
	NextRetry       string   `json:"nextRetry,omitempty"`
	Frozen          bool     `json:"frozen"`
	Restorable      bool     `json:"restorable"`
	NeedsSidecar    bool     `json:"needsSidecar"`
	Missing         []string `json:"missingLanguages,omitempty"`
	Searchable      bool     `json:"searchable"`
	ActionLanguage  string   `json:"actionLanguage,omitempty"`
}

type subtitleDashboardData struct {
	subtitleDashboardOptions
	PageSize           int                      `json:"pageSize"`
	Pages              int                      `json:"pages"`
	Matched            int                      `json:"matched"`
	Start              int                      `json:"start"`
	End                int                      `json:"end"`
	PreviousURL        string                   `json:"previousURL,omitempty"`
	NextURL            string                   `json:"nextURL,omitempty"`
	FilterURL          string                   `json:"-"`
	LoadFailed         bool                     `json:"-"`
	Pending            int                      `json:"pending"`
	Unavailable        int                      `json:"unavailable"`
	ServerName         string                   `json:"serverName"`
	Language           string                   `json:"language"`
	Languages          []string                 `json:"languages"`
	LanguageSummary    string                   `json:"languageSummary"`
	Total              int                      `json:"total"`
	Ready              int                      `json:"ready"`
	Wanted             int                      `json:"wanted"`
	Coverage           int                      `json:"coverage"`
	ProviderReady      bool                     `json:"providerReady"`
	ProviderConfigured bool                     `json:"providerConfigured"`
	ProviderAvailable  bool                     `json:"providerAvailable"`
	Preference         string                   `json:"preference"`
	Readiness          subtitleReadiness        `json:"readiness"`
	Providers          []subtitleProviderHealth `json:"providers"`
	Items              []subtitleDashboardItem  `json:"items"`
}

type subtitleManager struct {
	factFailures     sync.Map // Item ID to the source version whose track check failed.
	index            *libraryIndex
	settings         *settingsStore
	provider         *subtitleProvider
	probe            *mediaProbe
	backups          *backupManager
	drafts           subtitleDraftStore
	automationCursor int
}

func newSubtitleManager(index *libraryIndex, settings *settingsStore, provider *subtitleProvider, probe *mediaProbe, backups ...*backupManager) *subtitleManager {
	manager := &subtitleManager{index: index, settings: settings, provider: provider, probe: probe}
	if provider != nil && provider.sync != nil {
		provider.sync.probe = probe
	}
	if len(backups) > 0 {
		manager.backups = backups[0]
	}
	return manager
}

func (manager *subtitleManager) register(mux *http.ServeMux, auth *authentication) {
	manager.registerSubtitleReview(mux, auth)
	mux.Handle("POST /subtitles/manage/fetch-wanted", auth.owner(http.HandlerFunc(manager.fetchWantedWeb)))
	mux.Handle("POST /subtitles/manage/maintain", auth.owner(http.HandlerFunc(manager.maintainWeb)))
	mux.Handle("POST /subtitles/manage/{id}/fetch", auth.owner(http.HandlerFunc(manager.fetchWeb)))
	mux.Handle("POST /subtitles/manage/{id}/restore", auth.owner(http.HandlerFunc(manager.restoreWeb)))
	mux.Handle("POST /subtitles/manage/{id}/replacement", auth.owner(http.HandlerFunc(manager.replacementWeb)))
	mux.Handle("POST /subtitles/providers/test", auth.owner(http.HandlerFunc(manager.testProvidersWeb)))
	mux.HandleFunc("GET /static/subtitle-status.js", serveScript(subtitleStatusJS))
	mux.Handle("GET /api/v1/subtitle-library", auth.owner(http.HandlerFunc(manager.statusAPI)))
	mux.Handle("POST /api/v1/subtitle-library/fetch-wanted", auth.owner(http.HandlerFunc(manager.fetchWantedAPI)))
	mux.Handle("POST /api/v1/subtitle-library/maintain", auth.owner(http.HandlerFunc(manager.maintainAPI)))
	mux.Handle("POST /api/v1/subtitle-library/{id}/fetch", auth.owner(http.HandlerFunc(manager.fetchAPI)))
	mux.Handle("POST /api/v1/subtitle-library/{id}/restore", auth.owner(http.HandlerFunc(manager.restoreAPI)))
	mux.Handle("POST /api/v1/subtitle-library/{id}/replacement", auth.owner(http.HandlerFunc(manager.replacementAPI)))
	mux.Handle("POST /api/v1/subtitle-providers/test", auth.owner(http.HandlerFunc(manager.testProvidersAPI)))
}

func (manager *subtitleManager) dashboard(writer http.ResponseWriter, request *http.Request) {
	options, err := subtitleDashboardQuery(request)
	if err != nil {
		localizedPageError(writer, request, "Request could not be completed.", "Check the view and search values, then try again.", http.StatusBadRequest)
		return
	}
	// Send usable navigation before library, readiness, and recovery-file checks.
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("X-Accel-Buffering", "no")
	shell := subtitleDashboardData{ServerName: manager.settings.serverName(), subtitleDashboardOptions: options}
	if err := subtitleDashboardView.ExecuteTemplate(writer, request, "shell", shell); err != nil {
		return
	}
	if err := http.NewResponseController(writer).Flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return
	}
	if request.Context().Err() != nil {
		return
	}
	data, err := manager.projection(request, options)
	if err != nil {
		data = subtitleDashboardData{LoadFailed: true}
	}
	// Headers are already committed; report projection failures inside the page.
	_ = subtitleDashboardView.ExecuteTemplate(writer, request, "content", data)
}

func (manager *subtitleManager) subtitleSidecarLanguage(item library.Item, languages, tracks []string, ready bool) string {
	if !ready {
		return ""
	}
	for _, preferred := range languages {
		sidecarReady, _ := manager.sidecarPlanCoverage(item, preferred)
		if ready && !sidecarReady && subtitleTrackListed(tracks, "Embedded "+preferred) {
			return preferred
		}
	}
	return ""
}

func (manager *subtitleManager) addSubtitleDashboardRecord(item *subtitleDashboardItem, source library.Item, language string, ready bool) {
	record, found, _ := manager.provider.ledger.record(subtitleRecordKey(source.ID, language))
	if !found {
		return
	}
	item.Source, item.Score, item.Frozen = record.Source, record.Score, record.Frozen
	item.ReleaseMatch = strconv.Itoa(int(record.ReleaseMatch*100)) + "%"
	item.Cleanup, item.Synchronization = strings.Join(record.Cleanup, " · "), record.Synchronization
	if record.InstalledAt > 0 {
		item.Installed = time.Unix(record.InstalledAt, 0).UTC().Format(time.RFC3339)
	}
	if ready {
		item.WantedReason = "Covered by the preferred plan"
	}
}

func (manager *subtitleManager) addSubtitleDashboardSearch(item *subtitleDashboardItem, source library.Item, language string, ready bool) {
	search, found, _ := manager.provider.ledger.search(subtitleSearchKey(source.ID, language, manager.settings.subtitlePreference()))
	if !found {
		return
	}
	item.LastSearch = time.Unix(search.CheckedAt, 0).UTC().Format(time.RFC3339) + " · " + search.Outcome
	item.NextRetry = time.Unix(search.NextAt, 0).UTC().Format(time.RFC3339)
	if !ready && search.SafeError != "" {
		item.WantedReason = search.SafeError
	}
}

func subtitleMatches(item library.Item, query string) bool {
	query = strings.ToLower(query)
	return strings.Contains(strings.ToLower(item.Title), query) || strings.Contains(strings.ToLower(item.Show), query) || strings.Contains(strings.ToLower(item.ShowTitle), query) || strings.Contains(strings.ToLower(filepath.Base(item.Path)), query)
}

func twoDigits(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

func subtitleDashboardIdentity(item library.Item) subtitleDashboardItem {
	title, context, kind := item.Title, "", "movie"
	if item.Show != "" || item.ShowTitle != "" {
		title, kind = item.Show, "episode"
		if item.ShowTitle != "" {
			title = item.ShowTitle
		}
		episode := "S" + twoDigits(item.Season) + "E" + twoDigits(item.Episode)
		context = episode + " · " + strings.TrimPrefix(item.Title, episode+" · ")
	}
	file := strings.TrimPrefix(filepath.ToSlash(filepath.Join(item.Folder, filepath.Base(item.Path))), "./")
	return subtitleDashboardItem{ID: item.ID, Title: title, Context: context, File: file, Year: item.Year, MediaKind: kind, Artwork: item.Artwork != "" || item.ShowArtwork != ""}
}
