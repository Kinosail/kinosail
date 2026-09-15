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
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	markerlogic "github.com/MikeO7/kinosail/packages/markers"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/privatefile"
)

const probeCacheLimit = 512 << 10

type probeCacheEntry struct {
	Schema  int    `json:"schema"`
	Version string `json:"version"`
	Result  Result `json:"result"`
}

type probeCall struct {
	done   chan struct{}
	result Result
}

func New(executable string) *Probe {
	return &Probe{executable: executable, cache: make(map[string]Result), versions: make(map[string]string), calls: make(map[string]*probeCall)}
}

// ConfigureCache sets the installation-owned probe cache directory.
func (probe *Probe) ConfigureCache(directory string) {
	probe.mu.Lock()
	probe.cacheDir = directory
	probe.mu.Unlock()
}

// Inspect returns normalized media facts and applies app-owned enrichment.
func (probe *Probe) Inspect(ctx context.Context, item library.Item, enrichment Enrichment) Result {
	return probe.inspect(ctx, item, &enrichment)
}

// Facts returns cached media facts without chapter lookups or playback keyframe analysis.
func (probe *Probe) Facts(ctx context.Context, item library.Item) Result {
	return probe.inspect(ctx, item, nil)
}

// CachedFacts reads current media facts without starting or waiting for a probe.
func (probe *Probe) CachedFacts(item library.Item) (Result, bool) {
	return probe.cachedFacts(item, playback.SourceVersion(item.Path))
}

// MemoryScanFacts returns only facts already loaded by the background worker.
// It never reads either the media volume or the persistent probe cache.
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

// CachedScanFacts reads facts for the last library scan without touching media
// storage. Playback and mutations must use Facts to validate the live source.
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
		probe.cache[item.ID], probe.versions[item.ID] = result, version
		probe.mu.Unlock()
	}
	return result, found
}

func (probe *Probe) inspect(ctx context.Context, item library.Item, enrichment *Enrichment) Result {
	complete := func(result Result) Result { return probe.completeOptional(ctx, item, result, enrichment) }
	if probe.executable == "" {
		return complete(Result{})
	}
	version := playback.SourceVersion(item.Path)
	result, found := probe.cachedFacts(item, version)
	if found {
		return complete(result)
	}
	key := item.ID + "\x00" + version
	probe.mu.Lock()
	// A previous call can finish between the cache lookup and this lock.
	if cached, ok := probe.cache[item.ID]; ok && (probe.versions[item.ID] == "" || probe.versions[item.ID] == version) {
		probe.mu.Unlock()
		return complete(cached)
	}
	if call := probe.calls[key]; call != nil {
		probe.mu.Unlock()
		select {
		case <-call.done:
			return complete(call.result)
		case <-ctx.Done():
			return Result{}
		}
	}
	call := &probeCall{done: make(chan struct{})}
	probe.calls[key] = call
	probe.mu.Unlock()
	defer func() {
		probe.mu.Lock()
		call.result = result
		delete(probe.calls, key)
		close(call.done)
		probe.mu.Unlock()
	}()
	result, found = probe.run(ctx, item)
	if !found {
		return Result{}
	}
	probe.mu.Lock()
	probe.cache[item.ID], probe.versions[item.ID] = result, version
	probe.mu.Unlock()
	probe.save(item, version, result)
	return complete(result)
}

// Duration returns a source duration without app-owned enrichment.
func (probe *Probe) Duration(ctx context.Context, item library.Item) float64 {
	return probe.Facts(ctx, item).Duration
}

func (probe *Probe) run(ctx context.Context, item library.Item) (Result, bool) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	data, err := runProbe(ctx, probe.executable, "-v", "error", "-show_entries", "stream=index,codec_type,codec_name,profile,level,pix_fmt,field_order,width,height,r_frame_rate,sample_aspect_ratio,sample_rate,channels,channel_layout,color_range,color_space,color_transfer,color_primaries,bits_per_raw_sample:stream_disposition:stream_tags=language,title:stream_side_data:chapter=start_time,end_time:chapter_tags=title:format=format_name,duration,bit_rate:format_tags=title,artist,album_artist,album,genre,track,disc,replaygain_track_gain,replaygain_album_gain,r128_track_gain,r128_album_gain", "-of", "json", item.Path)
	if err != nil {
		return Result{}, false
	}
	return parse(data)
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
	// Chapter classifications can become more conservative without decoding again.
	entry.Result.Markers = markerlogic.DetectPlaybackMarkers(entry.Result.Chapters)
	return entry.Result, true
}

func (probe *Probe) save(item library.Item, version string, result Result) {
	path := probe.cachePath(item.ID)
	if path != "" && version != "missing" && validProbeResult(result) {
		if data, err := json.Marshal(probeCacheEntry{Schema: 3, Version: version, Result: result}); err == nil {
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
	return entry.Schema == 3 && entry.Version == version && validProbeResult(entry.Result)
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

func validVideoBounds(video playback.VideoFacts) bool { //nolint:cyclop // These independent persisted video fields share one flat bounds contract.
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

func (probe *Probe) completeOptional(ctx context.Context, item library.Item, result Result, enrichment *Enrichment) Result {
	if enrichment == nil {
		return result
	}
	return probe.complete(ctx, item, result, *enrichment)
}
