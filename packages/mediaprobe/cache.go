package mediaprobe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"path/filepath"
	"strconv"

	"github.com/MikeO7/kinosail/packages/library"
	markerlogic "github.com/MikeO7/kinosail/packages/markers"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/privatefile"
)

const probeCacheLimit = 512 << 10

type probeCacheEntry struct {
	Schema          int    `json:"schema"`
	Version         string `json:"version"`
	Result          Result `json:"result"`
	RandomAccessFor string `json:"randomAccessFor,omitempty"`
}

type probeCall struct {
	done    chan struct{}
	result  Result
	outcome probeOutcome
}

func New(executable string) *Probe {
	return &Probe{executable: executable, cache: make(map[string]Result), versions: make(map[string]string), calls: make(map[string]*probeCall), access: make(map[string]accessEntry), failures: make(map[string]probeFailure)}
}

func (probe *Probe) ConfigureCache(directory string) {
	probe.mu.RLock()
	configured := probe.cacheDir == directory
	probe.mu.RUnlock()
	if configured {
		return
	}
	probe.mu.Lock()
	probe.cacheDir = directory
	probe.mu.Unlock()
}

func (probe *Probe) Inspect(ctx context.Context, item library.Item, enrichment Enrichment) Result {
	return probe.inspect(ctx, item, &enrichment)
}

func (probe *Probe) Facts(ctx context.Context, item library.Item) Result {
	return probe.inspect(ctx, item, nil)
}

func (probe *Probe) CachedFacts(item library.Item) (Result, bool) {
	return probe.cachedFacts(item, playback.SourceVersion(item.Path))
}

func (probe *Probe) MemoryScanFacts(item library.Item) (Result, bool) {
	if item.Size < 0 || item.Added.IsZero() {
		return Result{}, false
	}
	version := strconv.FormatInt(item.Size, 10) + ":" + strconv.FormatInt(item.Added.UnixNano(), 10)
	probe.mu.RLock()
	defer probe.mu.RUnlock()
	result, found := probe.cache[item.ID]
	cachedVersion := probe.versions[item.ID]
	return result, found && (cachedVersion == "" || cachedVersion == version)
}

func (probe *Probe) CachedScanFacts(item library.Item) (Result, bool) {
	if item.Size < 0 || item.Added.IsZero() {
		return Result{}, false
	}
	version := strconv.FormatInt(item.Size, 10) + ":" + strconv.FormatInt(item.Added.UnixNano(), 10)
	return probe.cachedFacts(item, version)
}

func (probe *Probe) cachedFacts(item library.Item, version string) (Result, bool) {
	probe.mu.RLock()
	result, found := probe.cache[item.ID]
	cachedVersion := probe.versions[item.ID]
	probe.mu.RUnlock()
	if found && (cachedVersion == "" || cachedVersion == version) {
		return result, true
	}
	if result, found = probe.load(item, version); found {
		probe.mu.Lock()
		probe.remember(item.ID, version, result)
		probe.mu.Unlock()
	}
	return result, found
}

func (probe *Probe) inspect(ctx context.Context, item library.Item, enrichment *Enrichment) Result {
	if probe.executable == "" {
		return probe.completeOptional(ctx, item, "", Result{}, enrichment)
	}
	version := playback.SourceVersion(item.Path)
	complete := func(result Result) Result { return probe.completeOptional(ctx, item, version, result, enrichment) }
	if result, found := probe.cachedFacts(item, version); found {
		return complete(result)
	}
	key := item.ID + "\x00" + version
	for ctx.Err() == nil {
		probe.mu.Lock()
		if cached, ok := probe.cache[item.ID]; ok && (probe.versions[item.ID] == "" || probe.versions[item.ID] == version) {
			probe.mu.Unlock()
			return complete(cached)
		}
		if probe.failedRecently(item.ID, version) {
			probe.mu.Unlock()
			return Result{}
		}
		call := probe.calls[key]
		if call == nil {
			call = &probeCall{done: make(chan struct{})}
			probe.calls[key] = call
			probe.mu.Unlock()
			return probe.lead(ctx, item, version, key, call, complete)
		}
		probe.mu.Unlock()
		select {
		case <-call.done:
			if call.outcome == probeFound {
				return complete(call.result)
			}
			if call.outcome != probeInterrupted {
				return Result{}
			}
		case <-ctx.Done():
		}
	}
	return Result{Interrupted: true}
}

func (probe *Probe) lead(ctx context.Context, item library.Item, version, key string, call *probeCall, complete func(Result) Result) Result {
	result, outcome := probe.run(ctx, item)
	probe.mu.Lock()
	switch {
	case outcome == probeFound:
		delete(probe.failures, item.ID)
		probe.remember(item.ID, version, result)
	case outcome == probeBroken && ctx.Err() == nil:
		probe.recordFailure(item.ID, version)
	case ctx.Err() != nil:
		outcome = probeInterrupted
	}
	call.result, call.outcome = result, outcome
	delete(probe.calls, key)
	close(call.done)
	probe.mu.Unlock()
	if outcome != probeFound {
		logProbeFailure(ctx, item, outcome)
		return Result{Interrupted: outcome == probeInterrupted}
	}
	probe.save(item, version, result)
	return complete(result)
}

func (probe *Probe) Duration(ctx context.Context, item library.Item) float64 {
	return probe.Facts(ctx, item).Duration
}

func (probe *Probe) load(item library.Item, version string) (Result, bool) {
	path := probe.cachePath(item.ID)
	if path == "" || version == "missing" {
		return Result{}, false
	}
	data, err := privatefile.Read(path, probeCacheLimit)
	if err != nil {
		return Result{}, false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var entry probeCacheEntry
	if decoder.Decode(&entry) != nil || decoder.Decode(&struct{}{}) != io.EOF || !validProbeCache(entry, version) {
		return Result{}, false
	}
	entry.Result.Markers = markerlogic.DetectPlaybackMarkers(entry.Result.Chapters)
	if entry.RandomAccessFor != "" && len(entry.Result.RandomAccess) > 0 {
		probe.mu.Lock()
		probe.rememberAccess(item.ID, accessEntry{version: version, intervals: entry.RandomAccessFor, points: entry.Result.RandomAccess})
		probe.mu.Unlock()
	}
	entry.Result.RandomAccess = nil
	return entry.Result, true
}

func (probe *Probe) save(item library.Item, version string, result Result) {
	probe.saveEntry(item, probeCacheEntry{Schema: 3, Version: version, Result: result})
}

func (probe *Probe) saveEntry(item library.Item, entry probeCacheEntry) {
	path := probe.cachePath(item.ID)
	if path != "" && entry.Version != "missing" && len(entry.RandomAccessFor) <= 4096 && validProbeResult(entry.Result) {
		if data, err := json.Marshal(entry); err == nil {
			_ = privatefile.Write(path, data)
		}
	}
}

func (probe *Probe) cachePath(id string) string {
	probe.mu.RLock()
	defer probe.mu.RUnlock()
	if probe.cacheDir == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(id))
	return filepath.Join(probe.cacheDir, "probes", hex.EncodeToString(digest[:16])+".json")
}

func validProbeCache(entry probeCacheEntry, version string) bool {
	return entry.Schema == 3 && entry.Version == version && len(entry.RandomAccessFor) <= 4096 && validProbeResult(entry.Result)
}

func validProbeResult(result Result) bool {
	return validProbeBounds(result) && validVideoBounds(result.Video) && validChapterBounds(result.Chapters) && validMarkerBounds(result.Markers) && validRandomAccess(result.RandomAccess)
}

func validProbeBounds(result Result) bool {
	return finite(result.Duration) && result.Duration >= 0 && result.Duration <= 366*24*60*60 && result.Bitrate >= 0 && result.Bitrate <= 1_000_000_000_000 && validProbeCardinality(result)
}

func validProbeCardinality(result Result) bool {
	return len(result.Audio) <= 64 && len(result.AudioFacts) <= 64 && len(result.SubtitleFacts) <= 64 && len(result.Chapters) <= 4096 && len(result.Markers) <= 4096 && len(result.RandomAccess) <= 8192
}

func validVideoBounds(video playback.VideoFacts) bool {
	return video.Rotation >= -360 && video.Rotation <= 360 && video.Rotation%90 == 0 && video.DolbyVisionProfile >= 0 && video.DolbyVisionProfile <= 10 && video.DolbyVisionCompatibility >= 0 && video.DolbyVisionCompatibility <= 15 && oneOf(video.FieldOrder, "", "unknown", "progressive", "tt", "bb", "tb", "bt") && video.Width >= 0 && video.Width <= 16384 && video.Height >= 0 && video.Height <= 16384 && video.BitDepth >= 0 && video.BitDepth <= 64 && finite(video.FrameRate) && video.FrameRate >= 0 && video.FrameRate <= 1000
}

func validChapterBounds(chapters []Chapter) bool {
	for _, Chapter := range chapters {
		if !finite(Chapter.Start) || !finite(Chapter.End) || Chapter.Start < 0 || Chapter.End <= Chapter.Start {
			return false
		}
	}
	return true
}

func validMarkerBounds(markers []Marker) bool {
	for _, marker := range markers {
		if !finite(marker.Start) || !finite(marker.End) || marker.Start < 0 || marker.End <= marker.Start {
			return false
		}
	}
	return true
}

func validRandomAccess(points []float64) bool {
	for _, point := range points {
		if !finite(point) || point < 0 {
			return false
		}
	}
	return true
}

func (probe *Probe) completeOptional(ctx context.Context, item library.Item, version string, result Result, enrichment *Enrichment) Result {
	if enrichment == nil {
		return result
	}
	return probe.complete(ctx, item, version, result, *enrichment)
}
