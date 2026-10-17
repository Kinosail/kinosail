package markers

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/documentdb"
	"github.com/MikeO7/kinosail/packages/library"
)

type Record struct {
	Revision        string   `json:"revision"`
	DetectorVersion int      `json:"detectorVersion,omitempty"`
	Markers         []Marker `json:"markers"`
	Suppressed      []string `json:"suppressed,omitempty"`
	Failed          int64    `json:"failed,omitempty"`
}

const (
	DetectorVersion     = 3
	failedAnalysisRetry = 7 * 24 * time.Hour
)

type Analyzer struct {
	mu                        sync.RWMutex
	file, cache, ffmpeg, tool string
	records                   map[string]Record
	jobs                      chan markerJob
	targets, failed, deferred map[string]bool
	threads, workers          int
	saveEntries               func(string, map[string]any, []string) error
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

type Config struct {
	DataDir, CacheDir, FFmpeg, Fingerprint string
	Database                               *documentdb.Store
	Acquire                                func(context.Context) (func(), error)
	Threads, BackgroundSlots               int
}

type markerJob struct {
	items   []library.Item
	targets map[string]bool
}

type Media struct {
	Duration    float64
	Markers     []Marker
	Interrupted bool
}

func NewAnalyzer(config Config) *Analyzer {
	cache := ""
	if config.CacheDir != "" {
		cache = filepath.Join(config.CacheDir, "markers")
	}
	acquire := config.Acquire
	if acquire == nil {
		acquire = func(context.Context) (func(), error) { return func() {}, nil }
	}
	analyzer := &Analyzer{cache: cache, ffmpeg: config.FFmpeg, tool: config.Fingerprint, records: make(map[string]Record), jobs: make(chan markerJob, 1), acquire: acquire, state: "idle", persist: statePersistence(config.Database), threads: max(1, config.Threads), workers: max(1, min(2, runtime.GOMAXPROCS(0)-1))}
	if config.BackgroundSlots > 0 {
		analyzer.workers = max(1, min(analyzer.workers, config.BackgroundSlots-1))
	}
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
	} else if config.Database != nil && config.Database.EnableEntries(filepath.Base(analyzer.file)) == nil {
		analyzer.saveEntries = config.Database.SaveEntries
	}
	return analyzer
}

func (analyzer *Analyzer) Schedule(ctx context.Context, observe func(func([]library.Item)), probe func(context.Context, library.Item) Media) {
	if ctx == nil || observe == nil || probe == nil {
		return
	}
	analyzer.SetProbe(probe)
	observe(analyzer.enqueueChanged)
	go analyzer.work(ctx)
}

func (analyzer *Analyzer) SetProbe(probe func(context.Context, library.Item) Media) {
	analyzer.mu.Lock()
	analyzer.probe = probe
	analyzer.mu.Unlock()
}

func (analyzer *Analyzer) SetWait(wait func(context.Context) error) {
	analyzer.mu.Lock()
	analyzer.wait = wait
	analyzer.mu.Unlock()
}

func (analyzer *Analyzer) enqueueChanged(items []library.Item) {
	changed := analyzer.changedItems(items)
	if len(changed) == 0 {
		analyzer.mu.Lock()
		if analyzer.state == "idle" {
			analyzer.state = "complete"
		}
		analyzer.mu.Unlock()
		return
	}
	targets := make(map[string]bool, len(changed))
	movieLibraries := make(map[string]bool)
	for _, item := range changed {
		targets[item.ID] = true
		if item.Show == "" {
			movieLibraries[strings.ToLower(item.Library)] = true
		}
	}
	for _, item := range items {
		if item.Kind == "video" && item.Show == "" && !targets[item.ID] && movieLibraries[strings.ToLower(item.Library)] {
			changed = append(changed, item)
		}
	}
	analyzer.enqueue(markerJob{items: changed, targets: targets})
}

func (analyzer *Analyzer) changedItems(items []library.Item) []library.Item {
	analyzer.mu.RLock()
	defer analyzer.mu.RUnlock()
	dirtySeasons := make(map[string]bool)
	changed := make([]library.Item, 0)
	for _, item := range items {
		if item.Kind != "video" || analyzer.records[item.ID].current(item) {
			continue
		}
		if item.Show != "" {
			dirtySeasons[seasonKey(item)] = true
		} else {
			changed = append(changed, item)
		}
	}
	for _, item := range items {
		if item.Kind == "video" && item.Show != "" && dirtySeasons[seasonKey(item)] {
			changed = append(changed, item)
		}
	}
	return changed
}

func seasonKey(item library.Item) string {
	return strings.ToLower(item.Library + "\x00" + item.Show + "\x00" + strconv.Itoa(item.Season))
}

func (record Record) current(item library.Item) bool {
	if record.Revision != mediaRevision(item) || record.DetectorVersion != DetectorVersion {
		return false
	}
	return record.Failed == 0 || time.Since(time.Unix(record.Failed, 0)) < failedAnalysisRetry
}

func (analyzer *Analyzer) Enqueue(items []library.Item) {
	analyzer.enqueue(markerJob{items: append([]library.Item(nil), items...)})
}

func (analyzer *Analyzer) enqueue(job markerJob) {
	analyzer.mu.Lock()
	if analyzer.state != "running" {
		analyzer.state = "queued"
	}
	analyzer.mu.Unlock()
	select {
	case analyzer.jobs <- job:
	default:
		select {
		case <-analyzer.jobs:
		default:
		}
		select {
		case analyzer.jobs <- job:
		default:
		}
	}
}

func (analyzer *Analyzer) work(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-analyzer.jobs:
			wait := analyzer.waiter()
			if wait != nil {
				analyzer.setStatus("waiting", nil)
				if err := wait(ctx); err != nil {
					analyzer.setStatus("failed", err)
					continue
				}
			}
			analyzer.setStatus("running", nil)
			err := analyzer.analyzeBatch(ctx, job.items, job.targets)
			state := "complete"
			if err != nil {
				state = "failed"
			}
			analyzer.setStatus(state, err)
			if err != nil && !errors.Is(err, context.Canceled) {
				slog.Warn("playback marker analysis failed", "error", err)
			} else if err == nil {
				slog.Debug("playback marker analysis completed", "items", len(job.items))
			}
		}
	}
}
