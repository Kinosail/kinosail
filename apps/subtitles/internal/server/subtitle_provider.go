package server

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/library"
)

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
	subdl     *subDLProvider
	open      *openSubtitlesProvider
	subsource *subSourceProvider
	ledger    *subtitleLedger
	health    *subtitleProviderHealthRegistry
	admission *subtitleAdmission
	sidecar   sync.Mutex
	current   atomic.Pointer[subtitleProvider]
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

func newSubtitleProvider(config SubtitleConfig, cache, data string, index *libraryIndex, settings *settingsStore, ffmpeg string, databases ...*database.Store) *subtitleProvider {
	provider := &subtitleProvider{cache: cache, index: index, settings: settings, sync: newSubtitleSynchronizer(ffmpeg), ledger: newSubtitleLedger(data, databases...), health: newSubtitleProviderHealthRegistry(data)}
	provider.sync.cacheDir = cache
	provider.useConfig(config)
	return provider
}

func (provider *subtitleProvider) useConfig(config SubtitleConfig) {
	if config.URL == "" {
		config.URL = defaultSubDLURL
	}
	provider.config = config
	provider.subdl = newSubDLProvider(config.URL, config.APIKey, provider.health)
	provider.open = newOpenSubtitlesProvider(config.OpenSubtitles, provider.health)
	provider.subsource = newSubSourceProvider(config.SubSource, provider.health)
}

func (provider *subtitleProvider) active() *subtitleProvider {
	if current := provider.current.Load(); current != nil {
		return current
	}
	return provider
}

func (provider *subtitleProvider) replaceConfig(config SubtitleConfig) {
	previous := provider.active().config
	next := &subtitleProvider{cache: provider.cache, index: provider.index, settings: provider.settings, sync: provider.sync, ledger: provider.ledger, health: provider.health, admission: provider.admission}
	next.useConfig(config)
	if next.config == previous {
		return
	}
	provider.current.Store(next)
	for name, changed := range map[string]bool{
		"SubDL":         next.config.URL != previous.URL || next.config.APIKey != previous.APIKey,
		"OpenSubtitles": next.config.OpenSubtitles != previous.OpenSubtitles,
		"SubSource":     next.config.SubSource != previous.SubSource,
	} {
		if changed {
			provider.health.reset(name)
		}
	}
	provider.ledger.clearSearches()
}

func (provider *subtitleProvider) label() string {
	if provider.configured() {
		return "Credentials configured"
	}
	return "Not configured"
}

func (provider *subtitleProvider) configured() bool {
	provider = provider.active()
	if provider.cache == "" {
		return false
	}
	for _, source := range provider.sources() {
		if source.configured() {
			return true
		}
	}
	return false
}

func validSubtitleProviderEndpoint(value string) bool {
	if value == "" || len(value) > 2048 {
		return false
	}
	endpoint, err := url.Parse(value)
	return err == nil && validIntegrationEndpoint(endpoint)
}

func (provider *subtitleProvider) fetchSidecar(ctx context.Context, item library.Item, language string) error {
	ctx, settled, admissionErr := provider.admission.enter(ctx)
	if admissionErr != nil {
		return admissionErr
	}
	defer settled()
	canonical, err := validateSubtitleLanguages([]string{language})
	if item.Kind != "video" || err != nil {
		return errors.New("subtitle request is invalid")
	}
	language = canonical[0]
	standard, variant := subtitleSidecarPath(item, language), ""
	if provider.preference() == "sdh" {
		variant = "sdh"
	}
	target, err := provider.openSidecarAt(item, language, subtitleVariantPath(item, language, variant))
	if err != nil {
		return err
	}
	defer target.close()
	if _, err := target.root.Lstat(target.name); err == nil {
		provider.ledger.noteSearch(subtitleSearchKey(item.ID, language, provider.preference()), "exists", "A subtitle file for this language already exists", item, time.Now()) //nolint:contextcheck // Ledger commits must finish after client cancellation.
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, standardErr := target.root.Lstat(filepath.Base(standard))
	accept := func(candidate subtitleDownloadCandidate) bool {
		return candidate.HI || variant != "sdh" || errors.Is(standardErr, os.ErrNotExist)
	}
	cleaned, record, err := provider.acquire(ctx, item, language, accept)
	if err != nil {
		outcome, safeError := "no-result", "No trusted subtitle found"
		if errors.Is(err, errSubtitleProviderUnavailable) {
			outcome, safeError = "provider-error", "Providers are temporarily unavailable"
		}
		provider.ledger.noteSearch(subtitleSearchKey(item.ID, language, provider.preference()), outcome, safeError, item, time.Now())
		return err
	}
	if err = provider.retainSubtitleOriginal(cleaned.Original, &record); err != nil {
		return err
	}
	if variant == "sdh" && record.Role != "captions" {
		target.name = filepath.Base(standard)
	}
	if err = provider.installSidecar(target, sidecarInstall{Key: subtitleRecordKey(item.ID, language), Data: cleaned.Data, Record: record, Action: "added", Reason: "missing"}); err != nil { //nolint:contextcheck // Ledger commits must finish after client cancellation.
		return err
	}
	provider.ledger.noteSearch(subtitleSearchKey(item.ID, language, provider.preference()), "installed", "", item, time.Now())
	return nil
}

func (provider *subtitleProvider) acquire(ctx context.Context, item library.Item, language string, accept func(subtitleDownloadCandidate) bool) (cleanedSubtitle, subtitleRecord, error) {
	provider = provider.active()
	if !provider.configured() || !validLanguage(language) {
		return cleanedSubtitle{}, subtitleRecord{}, errors.New("subtitle provider is not configured")
	}
	candidates, configured, failed := provider.subtitleCandidates(ctx, item, language)
	if len(candidates) == 0 && configured > 0 && failed > 0 {
		return cleanedSubtitle{}, subtitleRecord{}, errSubtitleProviderUnavailable
	}
	candidates = provider.preferredSubtitleCandidates(candidates)
	return provider.downloadSubtitleCandidates(ctx, item, candidates, accept)
}

func (provider *subtitleProvider) subtitleCandidates(ctx context.Context, item library.Item, language string) ([]subtitleDownloadCandidate, int, int) {
	provider = provider.active()
	candidates := make([]subtitleDownloadCandidate, 0, maximumProviderCandidates*3)
	configured, failed := 0, 0
	searchContext, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	sources := provider.sources()
	results := make([]subtitleCandidateSearch, len(sources))
	var workers sync.WaitGroup
	for index, source := range sources {
		workers.Go(func() { results[index] = searchSubtitleSource(searchContext, source, item, language) })
	}
	workers.Wait()
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

func saveSubtitleExclusive(target string, data []byte) error {
	return writeAtomicFileExclusive(target, data)
}
