package metadata

import (
	"context"
	"fmt"
	"math"
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

const defaultChaptersDBURL = "https://chaptersdb.com/api/v1"

const (
	maxChaptersDBResponse = 1 << 20
	maxChaptersDBSets     = 32
	maxChaptersDBEntries  = 256
	maxChapterTitleBytes  = 512
)

type ChapterProvider struct {
	baseURL string
	client  *http.Client
	mu      sync.Mutex
	cached  map[string]chapterLookup
	calls   map[string]*chapterLookupCall
}

// Chapter describes one bounded media timeline section.
type Chapter struct {
	Index int     `json:"index"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Title string  `json:"title"`
}

// Timestamp formats the chapter start for the player view.
func (value Chapter) Timestamp() string {
	seconds := int(value.Start)
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds%3600/60, seconds%60)
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

type chapterLookup struct {
	chapters []Chapter
	found    bool
}

type chapterLookupCall struct {
	done     chan struct{}
	chapters []Chapter
	found    bool
}

type chaptersDBResponse struct {
	EpisodeID    string          `json:"episodeId"`
	EpisodeTitle string          `json:"episodeTitle"`
	Chapters     []chaptersDBSet `json:"chapters"`
	Count        int             `json:"count"`
}

type chaptersDBSet struct {
	ID           string            `json:"id"`
	Entries      []chaptersDBEntry `json:"entries"`
	Note         string            `json:"note"`
	Upvotes      int               `json:"upvotes"`
	Downvotes    int               `json:"downvotes"`
	UploaderName string            `json:"uploaderName"`
	CreatedAt    string            `json:"createdAt"`
}

type chaptersDBEntry struct {
	Time string `json:"time"`
	Name string `json:"name"`
}

func NewChapterProvider(rawURL string) *ChapterProvider {
	if rawURL == "" {
		rawURL = defaultChaptersDBURL
	}
	endpoint, err := url.Parse(rawURL)
	if err != nil || !validChaptersDBEndpoint(endpoint) {
		return nil
	}
	client := hardenedHTTPClient(5 * time.Second)
	ip := net.ParseIP(endpoint.Hostname())
	if endpoint.Hostname() == "localhost" || ip != nil && ip.IsLoopback() {
		client = localIntegrationHTTPClient(5 * time.Second)
	}
	return &ChapterProvider{baseURL: strings.TrimRight(rawURL, "/"), client: client, cached: make(map[string]chapterLookup), calls: make(map[string]*chapterLookupCall)}
}

func validChaptersDBEndpoint(endpoint *url.URL) bool {
	return validProviderEndpoint(endpoint, "chaptersdb.com", "/api/v1")
}

func (provider *ChapterProvider) Chapters(ctx context.Context, item library.Item, duration float64, local []Chapter) []Chapter {
	if provider == nil || !finite(duration) || duration <= 0 || !needsChapterDBLookup(local) {
		return local
	}
	id, ok := chaptersDBEpisodeID(item)
	if !ok {
		return local
	}
	external, found := provider.lookup(ctx, id, duration)
	if !found {
		return local
	}
	return mergeExternalChapters(local, external)
}

func chaptersDBEpisodeID(item library.Item) (string, bool) {
	if item.Show == "" || item.Episode <= 0 {
		return "", false
	}
	raw := strings.TrimSpace(item.ProviderIDs["tvdb"])
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || id <= 0 {
		return "", false
	}
	return strconv.FormatInt(id, 10), true
}

func needsChapterDBLookup(chapters []Chapter) bool {
	if len(chapters) == 0 {
		return true
	}
	for index, value := range chapters {
		if value.Title == "Chapter "+strconv.Itoa(index+1) {
			return true
		}
	}
	return false
}

func (provider *ChapterProvider) lookup(ctx context.Context, id string, duration float64) ([]Chapter, bool) {
	provider.mu.Lock()
	if result, found := provider.cached[id]; found {
		provider.mu.Unlock()
		return cloneChapters(result.chapters), result.found
	}
	if call := provider.calls[id]; call != nil {
		provider.mu.Unlock()
		select {
		case <-call.done:
			return cloneChapters(call.chapters), call.found
		case <-ctx.Done():
			return nil, false
		}
	}
	call := &chapterLookupCall{done: make(chan struct{})}
	provider.calls[id] = call
	provider.mu.Unlock()

	chapters, found := provider.fetch(ctx, id, duration)
	provider.mu.Lock()
	call.chapters, call.found = chapters, found
	delete(provider.calls, id)
	if found {
		provider.cached[id] = chapterLookup{chapters: cloneChapters(chapters), found: true}
	}
	close(call.done)
	provider.mu.Unlock()
	return chapters, found
}

func (provider *ChapterProvider) fetch(ctx context.Context, id string, duration float64) ([]Chapter, bool) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.baseURL+"/chapters/"+id, nil)
	if err != nil {
		return nil, false
	}
	request.Header.Set("Accept", "application/json")
	response, err := provider.client.Do(request)
	if err != nil {
		return nil, false
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, true
	}
	if response.StatusCode != http.StatusOK {
		return nil, false
	}
	var payload chaptersDBResponse
	if decodeExternalJSONStrict(response.Body, maxChaptersDBResponse, &payload) != nil {
		return nil, false
	}
	if len(payload.Chapters) > maxChaptersDBSets {
		return nil, false
	}
	for _, set := range payload.Chapters {
		chapters, ok := chaptersFromDBEntries(set.Entries, duration)
		if ok {
			return chapters, true
		}
	}
	return nil, true
}

func chaptersFromDBEntries(entries []chaptersDBEntry, duration float64) ([]Chapter, bool) { //nolint:cyclop // Entry validation and timeline construction must remain one atomic parse.
	if len(entries) == 0 || len(entries) > maxChaptersDBEntries || !finite(duration) || duration <= 0 {
		return nil, false
	}
	chapters := make([]Chapter, 0, len(entries))
	previous := -1.0
	for index, entry := range entries {
		start, ok := parseChapterDBTime(entry.Time)
		name := strings.TrimSpace(entry.Name)
		if !ok || start < 0 || start >= duration || start <= previous || len(name) == 0 || len(name) > maxChapterTitleBytes || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			return nil, false
		}
		chapters = append(chapters, Chapter{Index: index, Start: start, Title: name})
		previous = start
	}
	for index := range chapters {
		end := duration
		if index+1 < len(chapters) {
			end = chapters[index+1].Start
		}
		chapters[index].End = end
	}
	return chapters, true
}

func parseChapterDBTime(value string) (float64, bool) { //nolint:cyclop // The accepted clock components and bounds form one strict grammar.
	parts := strings.SplitN(strings.TrimSpace(value), ".", 2)
	clock := strings.Split(parts[0], ":")
	if len(clock) != 3 || len(clock[0]) == 0 || len(clock[0]) > 3 || len(clock[1]) != 2 || len(clock[2]) != 2 {
		return 0, false
	}
	hours, hourErr := strconv.Atoi(clock[0])
	minutes, minuteErr := strconv.Atoi(clock[1])
	seconds, secondErr := strconv.Atoi(clock[2])
	if hourErr != nil || minuteErr != nil || secondErr != nil || minutes > 59 || seconds > 59 {
		return 0, false
	}
	fraction := 0.0
	if len(parts) == 2 {
		if len(parts[1]) == 0 || len(parts[1]) > 3 {
			return 0, false
		}
		value, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, false
		}
		fraction = float64(value) / math.Pow10(len(parts[1]))
	}
	return float64(hours*3600+minutes*60+seconds) + fraction, true
}

func mergeExternalChapters(local, external []Chapter) []Chapter {
	if len(external) == 0 {
		return local
	}
	if len(local) == 0 {
		return cloneChapters(external)
	}
	if len(local) != len(external) {
		return local
	}
	result := cloneChapters(local)
	for index := 0; index < len(result) && index < len(external); index++ {
		if math.Abs(result[index].Start-external[index].Start) > 3 || result[index].Title != "Chapter "+strconv.Itoa(index+1) {
			continue
		}
		result[index].Title = external[index].Title
	}
	return result
}

func cloneChapters(chapters []Chapter) []Chapter {
	return append([]Chapter(nil), chapters...)
}
