package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

// SubtitleConfig configures subtitle adapters without persisting credentials.
type SubtitleConfig struct {
	URL, APIKey   string
	OpenSubtitles OpenSubtitlesConfig
	SubSource     SubSourceConfig
}

type subtitleProvider struct {
	config    SubtitleConfig
	cache     string
	index     *libraryIndex
	settings  *settingsStore
	sync      *subtitleSynchronizer
	open      *openSubtitlesProvider
	subsource *subSourceProvider
	client    *http.Client
	ledger    *subtitleLedger
	health    *subtitleProviderHealthRegistry
	sidecar   sync.Mutex
}

type subtitleDownloadCandidate struct {
	Language     string
	ExactHash    bool
	Score        int
	ReleaseMatch float64
	Source       string
	Download     func(context.Context) ([]byte, error)
	Preserve     bool
	HI           bool
}

type subtitleCandidateSearch struct {
	Candidates         []subtitleDownloadCandidate
	Configured, Failed bool
}

var (
	errNoTrustedSubtitle           = errors.New("no trusted subtitle found")
	errSubtitleProviderUnavailable = errors.New("subtitle providers are unavailable")
)

func newSubtitleProvider(config SubtitleConfig, cache, data string, index *libraryIndex, settings *settingsStore, ffmpeg string) *subtitleProvider {
	if config.URL == "" {
		config.URL = "https://api.subdl.com/api/v1"
	}
	health := newSubtitleProviderHealthRegistry(data)
	provider := &subtitleProvider{config: config, cache: cache, index: index, settings: settings, sync: newSubtitleSynchronizer(ffmpeg), open: newOpenSubtitlesProvider(config.OpenSubtitles), subsource: newSubSourceProvider(config.SubSource), ledger: newSubtitleLedger(data), health: health}
	provider.open.health, provider.subsource.health = health, health
	provider.client = localIntegrationHTTPClient(15 * time.Second)
	provider.client.CheckRedirect = func(request *http.Request, _ []*http.Request) error {
		if !provider.allowed(request.URL.String()) {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return provider
}

func (provider *subtitleProvider) label() string {
	if provider.configured() {
		return "Credentials configured"
	}
	return "Not configured"
}

func (provider *subtitleProvider) configured() bool {
	return provider.cache != "" && (provider.subDLConfigured() || provider.open.configured() || provider.subsource.configured())
}

func (provider *subtitleProvider) subDLConfigured() bool {
	return validSubtitleProviderEndpoint(provider.config.URL) && validSubtitleProviderCredential(provider.config.APIKey)
}

func validSubtitleProviderEndpoint(value string) bool {
	if value == "" || len(value) > 2048 {
		return false
	}
	endpoint, err := url.Parse(value)
	return err == nil && validIntegrationEndpoint(endpoint)
}

func (provider *subtitleProvider) register(mux *http.ServeMux, auth *authentication) {
	mux.Handle("POST /subtitles/{id}/fetch", auth.owner(provider.fetchHandler()))
	mux.HandleFunc("GET /subtitles/{id}/{language}", provider.serve)
}

func (provider *subtitleProvider) fetchHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !emptyMutationRequest(writer, request) {
			localizedError(writer, request, "subtitle request is invalid", http.StatusBadRequest)
			return
		}
		item, found := visibleItem(request, provider.index, request.PathValue("id"))
		if !found || item.Kind != "video" {
			localizedNotFound(writer, request)
			return
		}
		if err := provider.fetch(request.Context(), item, provider.settings.subtitleLanguage()); err != nil {
			localizedError(writer, request, "subtitle provider unavailable", http.StatusBadGateway)
			return
		}
		http.Redirect(writer, request, "/watch/"+item.ID, http.StatusSeeOther)
	}
}

func (provider *subtitleProvider) fetch(ctx context.Context, item library.Item, language string) error {
	canonical, err := validateSubtitleLanguages([]string{language})
	if err != nil {
		return errors.New("subtitle request is invalid")
	}
	language = canonical[0]
	cleaned, record, err := provider.acquire(ctx, item, language, nil)
	if err != nil {
		return err
	}
	if err := provider.retainSubtitleOriginal(cleaned.Original, &record); err != nil {
		return err
	}
	return saveSubtitle(provider.path(item.ID, language), cleaned.Data)
}

func (provider *subtitleProvider) fetchSidecar(ctx context.Context, item library.Item, language string) error {
	canonical, err := validateSubtitleLanguages([]string{language})
	if item.Kind != "video" || err != nil {
		return errors.New("subtitle request is invalid")
	}
	language = canonical[0]
	target := subtitleSidecarPath(item, language)
	if _, err := os.Lstat(target); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	cleaned, record, err := provider.acquire(ctx, item, language, nil)
	if err != nil {
		outcome, safeError := "no-result", "No trusted subtitle found"
		if errors.Is(err, errSubtitleProviderUnavailable) {
			outcome, safeError = "provider-error", "Providers are temporarily unavailable"
		}
		provider.ledger.noteSearch(subtitleSearchKey(item.ID, language, provider.preference()), outcome, safeError, time.Now())
		return err
	}
	if err = provider.retainSubtitleOriginal(cleaned.Original, &record); err != nil {
		return err
	}
	if err = saveSubtitleExclusive(target, cleaned.Data); err != nil {
		return err
	}
	record = completeSubtitleRecord(target, cleaned.Data, record)
	if err = provider.ledger.store(subtitleRecordKey(item.ID, language), record); err != nil {
		_ = os.Remove(target)
		return err
	}
	provider.ledger.noteSearch(subtitleSearchKey(item.ID, language, provider.preference()), "installed", "", time.Now())
	return nil
}

func (provider *subtitleProvider) acquire(ctx context.Context, item library.Item, language string, accept func(subtitleDownloadCandidate) bool) (cleanedSubtitle, subtitleRecord, error) {
	if !provider.configured() || !validLanguage(language) {
		return cleanedSubtitle{}, subtitleRecord{}, errors.New("subtitle provider is not configured")
	}
	candidates, configured, failed := provider.subtitleCandidates(ctx, item, language)
	if len(candidates) == 0 && configured > 0 && failed == configured {
		return cleanedSubtitle{}, subtitleRecord{}, errSubtitleProviderUnavailable
	}
	candidates = provider.preferredSubtitleCandidates(candidates)
	return provider.downloadSubtitleCandidates(ctx, item, candidates, accept)
}

func (provider *subtitleProvider) subtitleCandidates(ctx context.Context, item library.Item, language string) ([]subtitleDownloadCandidate, int, int) {
	candidates := make([]subtitleDownloadCandidate, 0, maximumDownloadTries*3)
	configured, failed := 0, 0
	searchContext, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	searches := []func(context.Context, library.Item, string) subtitleCandidateSearch{
		provider.subDLCandidateSearch, provider.openSubtitlesCandidateSearch, provider.subSourceCandidateSearch,
	}
	results := make([]subtitleCandidateSearch, len(searches))
	var workers sync.WaitGroup
	for index, search := range searches {
		workers.Go(func() { results[index] = search(searchContext, item, language) })
	}
	workers.Wait()
	// Preserve provider order for deterministic ranking of equal candidates.
	for _, search := range results {
		candidates = append(candidates, search.Candidates...)
		if search.Configured {
			configured++
		}
		if search.Failed {
			failed++
		}
	}
	return candidates, configured, failed
}

func (provider *subtitleProvider) preferredSubtitleCandidates(candidates []subtitleDownloadCandidate) []subtitleDownloadCandidate {
	preferSDH := provider.preference() == "sdh"
	sort.SliceStable(candidates, func(left, right int) bool {
		leftScore := candidates[left].Score + subtitlePreferenceBonus(candidates[left].HI, preferSDH)
		rightScore := candidates[right].Score + subtitlePreferenceBonus(candidates[right].HI, preferSDH)
		if leftScore == rightScore {
			return candidates[left].ReleaseMatch > candidates[right].ReleaseMatch
		}
		return leftScore > rightScore
	})
	if len(candidates) > maximumDownloadTries {
		candidates = candidates[:maximumDownloadTries]
	}
	return candidates
}

func subtitlePreferenceBonus(hearingImpaired, preferSDH bool) int {
	if hearingImpaired == preferSDH {
		return 10
	}
	return 0
}

func (provider *subtitleProvider) preference() string {
	if provider.settings == nil {
		return "standard"
	}
	return provider.settings.subtitlePreference()
}

func saveSubtitle(target string, data []byte) error {
	return writeAtomicFile(target, data)
}

func saveSubtitleExclusive(target string, data []byte) error {
	return writeAtomicFileExclusive(target, data)
}
