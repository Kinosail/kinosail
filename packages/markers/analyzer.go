package markers

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/MikeO7/kinosail/packages/documentdb"
	"github.com/MikeO7/kinosail/packages/library"
)

type Record struct {
	Revision        string   `json:"revision"`
	DetectorVersion int      `json:"detectorVersion,omitempty"`
	Markers         []Marker `json:"markers"`
	Suppressed      []string `json:"suppressed,omitempty"`
}

const DetectorVersion = 3

type Analyzer struct {
	mu                        sync.RWMutex
	file, cache, ffmpeg, tool string
	records                   map[string]Record
	jobs                      chan []library.Item
	probe                     func(context.Context, library.Item) Media
	extract                   func(context.Context, library.Item, string, float64, float64) ([]uint32, error)
	extractVisual             func(context.Context, library.Item, string, float64, float64) ([]uint64, error)
	extractCredits            func(context.Context, library.Item, float64, float64) ([]byte, error)
	wait                      func(context.Context) error
	acquire                   func(context.Context) (func(), error)
	state                     string
	err                       error
	persist                   func(string, any) error
}

type episodeFingerprint struct {
	item          library.Item
	duration      float64
	opening       []uint32
	tail          []uint32
	openingVisual []uint64
	tailVisual    []uint64
	tailOffset    float64
	introAudio    markerCandidate
	introVisual   markerCandidate
	outroAudio    markerCandidate
	outroVisual   markerCandidate
}

type markerCandidate struct {
	ranges []timeRange
}

// Config contains the local resources used by marker analysis.
type Config struct {
	DataDir, CacheDir, FFmpeg, Fingerprint string
	Database                               *documentdb.Store
	Acquire                                func(context.Context) (func(), error)
}

// Media contains the probe facts required by marker analysis.
type Media struct {
	Duration float64
	Markers  []Marker
}

// NewAnalyzer returns a bounded playback marker analyzer.
func NewAnalyzer(config Config) *Analyzer {
	cache := ""
	if config.CacheDir != "" {
		cache = filepath.Join(config.CacheDir, "markers")
	}
	acquire := config.Acquire
	if acquire == nil {
		acquire = func(context.Context) (func(), error) { return func() {}, nil }
	}
	analyzer := &Analyzer{cache: cache, ffmpeg: config.FFmpeg, tool: config.Fingerprint, records: make(map[string]Record), jobs: make(chan []library.Item, 1), acquire: acquire, state: "idle", persist: statePersistence(config.Database)}
	if config.DataDir == "" {
		return analyzer
	}
	analyzer.file = filepath.Join(config.DataDir, "playback_markers.json")
	var found bool
	found, analyzer.err = loadState(config.Database, analyzer.file, &analyzer.records)
	if analyzer.err == nil && found {
		analyzer.err = validateRecords(analyzer.records)
	}
	if analyzer.err != nil {
		analyzer.records = make(map[string]Record)
	}
	return analyzer
}

// Schedule connects the analyzer to one library and probe adapter.
func (analyzer *Analyzer) Schedule(ctx context.Context, observe func(func([]library.Item)), probe func(context.Context, library.Item) Media) {
	if ctx == nil || observe == nil || probe == nil {
		return
	}
	analyzer.SetProbe(probe)
	observe(analyzer.enqueueChanged)
	go analyzer.work(ctx)
}

// SetProbe supplies the media facts used by direct and scheduled analysis.
func (analyzer *Analyzer) SetProbe(probe func(context.Context, library.Item) Media) {
	analyzer.mu.Lock()
	analyzer.probe = probe
	analyzer.mu.Unlock()
}

// SetWait supplies an optional idle gate for background analysis.
func (analyzer *Analyzer) SetWait(wait func(context.Context) error) {
	analyzer.mu.Lock()
	analyzer.wait = wait
	analyzer.mu.Unlock()
}

func (analyzer *Analyzer) enqueueChanged(items []library.Item) {
	items = analyzer.changedItems(items)
	if len(items) == 0 {
		analyzer.mu.Lock()
		if analyzer.state == "idle" {
			analyzer.state = "complete"
		}
		analyzer.mu.Unlock()
		return
	}
	analyzer.Enqueue(items)
}

func (analyzer *Analyzer) changedItems(items []library.Item) []library.Item { //nolint:cyclop // Movie revisions and season-wide episode invalidation are evaluated together.
	analyzer.mu.RLock()
	defer analyzer.mu.RUnlock()
	dirtySeasons := make(map[string]bool)
	dirtyMovieLibraries := make(map[string]bool)
	for _, item := range items {
		record, found := analyzer.records[item.ID]
		if item.Kind != "video" || found && record.Revision == mediaRevision(item) && record.DetectorVersion == DetectorVersion {
			continue
		}
		if item.Show != "" {
			dirtySeasons[strings.ToLower(item.Library+"\x00"+item.Show+"\x00"+strconv.Itoa(item.Season))] = true
		} else {
			dirtyMovieLibraries[strings.ToLower(item.Library)] = true
		}
	}
	changed := make([]library.Item, 0)
	for _, item := range items {
		season := strings.ToLower(item.Library + "\x00" + item.Show + "\x00" + strconv.Itoa(item.Season))
		if item.Kind == "video" && (item.Show != "" && dirtySeasons[season] || item.Show == "" && dirtyMovieLibraries[strings.ToLower(item.Library)]) {
			changed = append(changed, item)
		}
	}
	return changed
}

func (analyzer *Analyzer) Enqueue(items []library.Item) {
	items = append([]library.Item(nil), items...)
	analyzer.mu.Lock()
	if analyzer.state != "running" {
		analyzer.state = "queued"
	}
	analyzer.mu.Unlock()
	select {
	case analyzer.jobs <- items:
	default:
		select {
		case <-analyzer.jobs:
		default:
		}
		select {
		case analyzer.jobs <- items:
		default:
		}
	}
}

func (analyzer *Analyzer) work(ctx context.Context) { //nolint:cyclop,gocognit // The worker owns one bounded analysis lifecycle.
	for {
		select {
		case <-ctx.Done():
			return
		case items := <-analyzer.jobs:
			wait := analyzer.waiter()
			if wait != nil {
				analyzer.setStatus("waiting", nil)
				if err := wait(ctx); err != nil {
					analyzer.setStatus("failed", err)
					continue
				}
			}
			analyzer.setStatus("running", nil)
			release, err := analyzer.acquire(ctx)
			if err == nil {
				err = analyzer.analyzeBatch(ctx, items)
				release()
			}
			state := "complete"
			if err != nil {
				state = "failed"
			}
			analyzer.setStatus(state, err)
			if err != nil && !errors.Is(err, context.Canceled) {
				slog.Warn("playback marker analysis failed", "error", err)
			} else if err == nil {
				slog.Debug("playback marker analysis completed", "items", len(items))
			}
		}
	}
}

func (analyzer *Analyzer) analyzeSeason(ctx context.Context, items []library.Item) error {
	sort.Slice(items, func(i, j int) bool { return items[i].Episode < items[j].Episode })
	episodes := make([]episodeFingerprint, 0, len(items))
	for _, item := range items {
		media := analyzer.inspect(ctx, item)
		if !finite(media.Duration) || media.Duration <= 0 {
			return errors.New("playback segment media duration is unavailable")
		}
		openingLength := min(media.Duration*.25, 600)
		tailLength := min(media.Duration*.2, 900)
		opening, audioErr := analyzer.fingerprint(ctx, item, "opening", 0, openingLength)
		openingVisual, visualErr := analyzer.visualFingerprint(ctx, item, "opening", 0, openingLength)
		if audioErr != nil && visualErr != nil {
			return errors.New("playback opening analysis is unavailable")
		}
		tailOffset := media.Duration - tailLength
		tail, audioErr := analyzer.fingerprint(ctx, item, "tail", tailOffset, tailLength)
		tailVisual, visualErr := analyzer.visualFingerprint(ctx, item, "tail", tailOffset, tailLength)
		if audioErr != nil && visualErr != nil {
			return errors.New("playback ending analysis is unavailable")
		}
		episodes = append(episodes, episodeFingerprint{item: item, duration: media.Duration, opening: opening, tail: tail, openingVisual: openingVisual, tailVisual: tailVisual, tailOffset: tailOffset})
	}
	for _, pair := range markerCandidatePairs(episodes) {
		if err := ctx.Err(); err != nil {
			return err
		}
		analyzer.comparePair(&episodes[pair[0]], &episodes[pair[1]])
	}
	for _, episode := range episodes {
		markers := make([]Marker, 0, 2)
		intro, source, ok := strongestRangeFromCandidates(episode.introAudio, episode.introVisual)
		if ok {
			markers = append(markers, Marker{Type: "intro", Label: "Intro", Start: intro.Start, End: intro.End, Source: source})
		}
		outro, source, ok := strongestRangeFromCandidates(episode.outroAudio, episode.outroVisual)
		if ok {
			markers = append(markers, Marker{Type: "outro", Label: "Outro", Start: outro.Start, End: outro.End, Source: source})
		}
		analyzer.replaceDetected(episode.item, []string{"intro", "outro"}, markers)
	}
	return ctx.Err()
}
