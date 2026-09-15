package metadata

import (
	"context"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/MikeO7/kinosail/packages/library"
)

const defaultTVMazeURL = "https://api.tvmaze.com"

type TVMazeProvider struct {
	baseURL   string
	userAgent string
	client    *http.Client
	mu        sync.Mutex
	shows     map[string]int
	details   map[int]tvmazeShow
	episodes  map[string]tvmazeEpisode
}
type tvmazeShow struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Premiered string `json:"premiered"`
	Summary   string `json:"summary"`
}
type tvmazeEpisode struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Season  int    `json:"season"`
	Number  int    `json:"number"`
	Airdate string `json:"airdate"`
	Summary string `json:"summary"`
}

func NewTVMazeProvider(rawURL string, versions ...string) *TVMazeProvider { //nolint:cyclop // Endpoint and optional user-agent validation belong to construction.
	if rawURL == "" {
		rawURL = defaultTVMazeURL
	}
	endpoint, err := url.Parse(rawURL)
	if err != nil || !ValidTVMazeEndpoint(endpoint) {
		return nil
	}
	client := hardenedHTTPClient(5 * time.Second)
	ip := net.ParseIP(endpoint.Hostname())
	if endpoint.Hostname() == "localhost" || ip != nil && ip.IsLoopback() {
		client = localIntegrationHTTPClient(5 * time.Second)
	}
	version := "dev"
	if len(versions) == 1 && versions[0] != "" && len(versions[0]) <= 128 && !hasControlText(versions[0]) {
		version = versions[0]
	}
	return &TVMazeProvider{baseURL: strings.TrimRight(rawURL, "/"), userAgent: "Kinosail/" + version, client: client, shows: make(map[string]int), details: make(map[int]tvmazeShow), episodes: make(map[string]tvmazeEpisode)}
}

// ValidTVMazeEndpoint reports whether endpoint is official or local test infrastructure.
func ValidTVMazeEndpoint(endpoint *url.URL) bool {
	return validProviderEndpoint(endpoint, "api.tvmaze.com", "")
}

func (provider *TVMazeProvider) Record(ctx context.Context, item library.Item) (Result, bool) { //nolint:cyclop // Identity, episode, show, and record bounds form one provider transaction.
	tvdbID, ok := tvmazeTVDBID(item)
	if !ok || item.Episode <= 0 || item.Season < 0 {
		return Result{}, false
	}
	showID, ok := provider.showID(ctx, tvdbID)
	if !ok {
		return Result{}, false
	}
	episode, ok := provider.episode(ctx, showID, item.Season, item.Episode)
	if !ok || !validTVMazeEpisode(episode, item) {
		return Result{}, false
	}
	name := cleanTVMazeText(episode.Name)
	plot := cleanTVMazeText(episode.Summary)
	if name == "" || len(name) > 200 || len(plot) > 5000 || hasControlText(name) || hasInvalidRecordText(plot, true) {
		return Result{}, false
	}
	showTitle := item.ShowTitle
	showYear, showPlot := item.ShowYear, ""
	if show, found := provider.show(ctx, showID); found {
		showTitle = cleanTVMazeText(show.Name)
		showYear, showPlot = Year(show.Premiered), cleanTVMazeText(show.Summary)
	}
	if showTitle == "" {
		showTitle = item.Show
	}
	record := Record{Title: fmt.Sprintf("S%02dE%02d · %s", item.Season, item.Episode, name), Plot: plot, Year: Year(episode.Airdate), ShowTitle: showTitle, ShowYear: showYear, ShowPlot: showPlot, ProviderIDs: map[string]string{"tvdb": tvdbID, "tvmaze": strconv.Itoa(episode.ID)}, ShowProviderIDs: map[string]string{"tvmaze": strconv.Itoa(showID)}}
	if !boundedMetadata(record) {
		return Result{}, false
	}
	return Result{Record: record}, true
}

func (provider *TVMazeProvider) Fill(ctx context.Context, item library.Item, record *Record) { //nolint:cyclop // Fill preserves each local field while merging one validated provider record.
	result, ok := provider.Record(ctx, item)
	if !ok {
		return
	}
	if record.Title == "" {
		record.Title = result.Record.Title
	}
	if record.Plot == "" {
		record.Plot = result.Record.Plot
	}
	if record.Year == "" {
		record.Year = result.Record.Year
	}
	if record.ShowTitle == "" {
		record.ShowTitle = result.Record.ShowTitle
	}
	if record.ShowYear == "" {
		record.ShowYear = result.Record.ShowYear
	}
	if record.ShowPlot == "" {
		record.ShowPlot = result.Record.ShowPlot
	}
	if record.ProviderIDs == nil {
		record.ProviderIDs = make(map[string]string)
	}
	for key, value := range result.Record.ProviderIDs {
		if record.ProviderIDs[key] == "" {
			record.ProviderIDs[key] = value
		}
	}
	if record.ShowProviderIDs == nil {
		record.ShowProviderIDs = make(map[string]string)
	}
	if record.ShowProviderIDs["tvmaze"] == "" {
		record.ShowProviderIDs["tvmaze"] = result.Record.ShowProviderIDs["tvmaze"]
	}
}

func tvmazeTVDBID(item library.Item) (string, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(item.ProviderIDs["tvdb"]), 10, 32)
	return strconv.FormatInt(id, 10), err == nil && id > 0
}

func TVMazeEligible(item library.Item) bool {
	_, ok := tvmazeTVDBID(item)
	return item.Show != "" && item.Episode > 0 && item.Season >= 0 && ok
}

func (provider *TVMazeProvider) showID(ctx context.Context, tvdbID string) (int, bool) { //nolint:cyclop // Redirect status, location, path, and ID are one trust-boundary parse.
	provider.mu.Lock()
	if id := provider.shows[tvdbID]; id > 0 {
		provider.mu.Unlock()
		return id, true
	}
	provider.mu.Unlock()
	endpoint := provider.baseURL + "/lookup/shows?thetvdb=" + url.QueryEscape(tvdbID)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, false
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", provider.userAgent)
	response, err := provider.client.Do(request)
	if err != nil {
		return 0, false
	}
	defer response.Body.Close()
	location := response.Header.Get("Location")
	if response.StatusCode < http.StatusMultipleChoices || response.StatusCode >= http.StatusBadRequest || location == "" {
		return 0, false
	}
	parsed, err := url.Parse(location)
	if err != nil || !validTVMazeLocation(parsed) {
		return 0, false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "shows" {
		return 0, false
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil || id <= 0 {
		return 0, false
	}
	provider.mu.Lock()
	provider.shows[tvdbID] = id
	provider.mu.Unlock()
	return id, true
}

func (provider *TVMazeProvider) show(ctx context.Context, id int) (tvmazeShow, bool) {
	provider.mu.Lock()
	if show, found := provider.details[id]; found {
		provider.mu.Unlock()
		return show, true
	}
	provider.mu.Unlock()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/shows/%d", provider.baseURL, id), nil)
	if err != nil {
		return tvmazeShow{}, false
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", provider.userAgent)
	response, err := provider.client.Do(request)
	if err != nil {
		return tvmazeShow{}, false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return tvmazeShow{}, false
	}
	var show tvmazeShow
	if decodeExternalJSON(response.Body, 256<<10, &show) != nil || show.ID != id {
		return tvmazeShow{}, false
	}
	provider.mu.Lock()
	provider.details[id] = show
	provider.mu.Unlock()
	return show, true
}

func validTVMazeLocation(location *url.URL) bool {
	if location.RawQuery != "" || location.Fragment != "" {
		return false
	}
	return location.Host == "" || location.Scheme == "https" && location.Hostname() == "www.tvmaze.com" && location.Port() == ""
}

func (provider *TVMazeProvider) episode(ctx context.Context, showID, season, number int) (tvmazeEpisode, bool) {
	key := fmt.Sprintf("%d/%d/%d", showID, season, number)
	provider.mu.Lock()
	if episode, found := provider.episodes[key]; found {
		provider.mu.Unlock()
		return episode, true
	}
	provider.mu.Unlock()
	endpoint := fmt.Sprintf("%s/shows/%d/episodebynumber?season=%d&number=%d", provider.baseURL, showID, season, number)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return tvmazeEpisode{}, false
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", provider.userAgent)
	response, err := provider.client.Do(request)
	if err != nil {
		return tvmazeEpisode{}, false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return tvmazeEpisode{}, false
	}
	var episode tvmazeEpisode
	if decodeExternalJSON(response.Body, 256<<10, &episode) != nil {
		return tvmazeEpisode{}, false
	}
	provider.mu.Lock()
	provider.episodes[key] = episode
	provider.mu.Unlock()
	return episode, true
}

func validTVMazeEpisode(episode tvmazeEpisode, item library.Item) bool {
	return episode.ID > 0 && episode.Season == item.Season && episode.Number == item.Episode && len(episode.Airdate) <= 10
}

func cleanTVMazeText(value string) string {
	value = html.UnescapeString(value)
	var text strings.Builder
	inside := false
	for _, character := range value {
		switch character {
		case '<':
			inside = true
		case '>':
			inside = false
		default:
			if !inside {
				text.WriteRune(character)
			}
		}
	}
	return strings.Join(strings.Fields(text.String()), " ")
}

func hasControlText(value string) bool { return strings.IndexFunc(value, unicode.IsControl) >= 0 }
