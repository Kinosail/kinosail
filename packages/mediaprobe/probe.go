package mediaprobe

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	markerlogic "github.com/MikeO7/kinosail/packages/markers"
	"github.com/MikeO7/kinosail/packages/playback"
)

type (
	VideoFacts    = playback.VideoFacts
	AudioFacts    = playback.AudioFacts
	SubtitleFacts = playback.SubtitleFacts
)

type AudioTrack struct {
	Index int    `json:"index"`
	Label string `json:"label"`
}

type Probe struct {
	executable    string
	cacheDir      string
	mu            sync.RWMutex
	cache         map[string]Result
	versions      map[string]string
	calls         map[string]*probeCall
	embeddedCalls map[string]*embeddedCall
}

// Enrichment connects probe results to app-owned chapter and marker stores.
type Enrichment struct {
	Chapters func(context.Context, library.Item, float64, []Chapter) []Chapter
	Markers  func(library.Item, []Marker) []Marker
}

type Result struct {
	Summary       string
	Container     string
	Bitrate       int64
	Video         VideoFacts
	VideoCodec    string
	AudioCodec    string
	Audio         []AudioTrack
	AudioFacts    []AudioFacts
	SubtitleFacts []SubtitleFacts
	Duration      float64
	Chapters      []Chapter
	Markers       []Marker
	RandomAccess  []float64
	Tags          Tags
	ReplayGain    ReplayGain
}

// MediaFacts adapts one probe result to the shared playback decision model.
func (result Result) MediaFacts(item library.Item, fileVersion string, subtitleLanguage func(string, string) string, subtitleRole func(string) string) playback.MediaFacts {
	container := result.Container
	if container == "" {
		container = playback.Lower(item.Container)
	}
	facts := playback.MediaFacts{Kind: item.Kind, FileVersion: fileVersion, Container: playback.NormalizeContainer(container), Bitrate: result.Bitrate, Duration: result.Duration, Seekable: true, Video: result.Video, Audio: append([]AudioFacts(nil), result.AudioFacts...), Subtitles: append([]SubtitleFacts(nil), result.SubtitleFacts...), RandomAccess: append([]float64(nil), result.RandomAccess...)}
	for externalIndex, path := range item.Subtitles {
		index := len(facts.Subtitles)
		facts.Subtitles = append(facts.Subtitles, SubtitleFacts{Index: index, SourceIndex: -1, Codec: playback.Lower(strings.TrimPrefix(filepath.Ext(path), ".")), Language: subtitleLanguage(item.Path, path), Role: subtitleRole(path), Text: true, External: true, ExternalIndex: externalIndex})
	}
	return facts
}

type Tags struct {
	Title, Artist, AlbumArtist, Album, Genres string
	Disc, Track                               int
}

type probeOutput struct {
	Streams  []probeStream `json:"streams"`
	Chapters []struct {
		Start string            `json:"start_time"`
		End   string            `json:"end_time"`
		Tags  map[string]string `json:"tags"`
	} `json:"chapters"`
	Format struct {
		Name     string            `json:"format_name"`
		Duration string            `json:"duration"`
		Bitrate  string            `json:"bit_rate"`
		Tags     map[string]string `json:"tags"`
	} `json:"format"`
}

type probeStream struct {
	Index            int               `json:"index"`
	CodecType        string            `json:"codec_type"`
	CodecName        string            `json:"codec_name"`
	Profile          string            `json:"profile"`
	Level            int               `json:"level"`
	PixelFormat      string            `json:"pix_fmt"`
	Width            int               `json:"width"`
	Height           int               `json:"height"`
	FrameRate        string            `json:"r_frame_rate"`
	FieldOrder       string            `json:"field_order"`
	SampleAspect     string            `json:"sample_aspect_ratio"`
	SampleRate       string            `json:"sample_rate"`
	Channels         int               `json:"channels"`
	ChannelLayout    string            `json:"channel_layout"`
	ColorRange       string            `json:"color_range"`
	ColorSpace       string            `json:"color_space"`
	ColorTransfer    string            `json:"color_transfer"`
	Primaries        string            `json:"color_primaries"`
	BitsPerRawSample string            `json:"bits_per_raw_sample"`
	Tags             map[string]string `json:"tags"`
	Disposition      probeDisposition  `json:"disposition"`
	SideData         []probeSideData   `json:"side_data_list"`
}

type probeDisposition struct {
	Default         int `json:"default"`
	Forced          int `json:"forced"`
	HearingImpaired int `json:"hearing_impaired"`
	VisualImpaired  int `json:"visual_impaired"`
	Commentary      int `json:"comment"`
}

type probeSideData struct {
	Type            string `json:"side_data_type"`
	Rotation        int    `json:"rotation"`
	DVProfile       int    `json:"dv_profile"`
	DVCompatibility int    `json:"dv_bl_signal_compatibility_id"`
}

type randomAccessOutput struct {
	Frames []struct {
		KeyFrame int    `json:"key_frame"`
		Time     string `json:"best_effort_timestamp_time"`
	} `json:"frames"`
}

func (probe *Probe) complete(ctx context.Context, item library.Item, result Result, enrichment Enrichment) Result {
	if enrichment.Chapters != nil {
		chapters := enrichment.Chapters(ctx, item, result.Duration, result.Chapters)
		if len(chapters) != len(result.Chapters) || !sameChapterTitles(chapters, result.Chapters) {
			result.Chapters = chapters
			result.Markers = markerlogic.DetectPlaybackMarkers(chapters)
		}
	}
	if enrichment.Markers != nil {
		result.Markers = enrichment.Markers(item, result.Markers)
	}
	intervals := randomAccessIntervals(result.Markers, result.Duration)
	if item.Kind != "video" || intervals == "" || result.RandomAccess != nil {
		return result
	}
	result.RandomAccess = probe.randomAccess(ctx, item.Path, intervals)
	probe.mu.Lock()
	cached := probe.cache[item.ID]
	cached.RandomAccess = result.RandomAccess
	probe.cache[item.ID] = cached
	probe.mu.Unlock()
	return result
}

func sameChapterTitles(left, right []Chapter) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Title != right[index].Title {
			return false
		}
	}
	return true
}

func (probe *Probe) randomAccess(ctx context.Context, path, intervals string) []float64 {
	points := make([]float64, 0)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	data, err := runProbe(ctx, probe.executable, "-v", "error", "-skip_frame", "nokey", "-select_streams", "v:0", "-read_intervals", intervals, "-show_frames", "-show_entries", "frame=key_frame,best_effort_timestamp_time", "-of", "json", path)
	var output randomAccessOutput
	if err != nil || json.Unmarshal(data, &output) != nil {
		return points
	}
	for _, frame := range output.Frames {
		if value, parseErr := strconv.ParseFloat(frame.Time, 64); frame.KeyFrame == 1 && parseErr == nil && value >= 0 {
			points = append(points, value)
		}
	}
	return points
}

func randomAccessIntervals(markers []Marker, duration float64) string { //nolint:gocognit,cyclop // Validation, bounding, sorting, and range merging form one operation.
	const padding, maximumBoundaries = 2.0, 64
	if !finite(duration) || duration <= 0 {
		return ""
	}
	boundaries := make([]float64, 0, min(len(markers)*2, maximumBoundaries))
	for _, marker := range markers {
		if !automaticallySkippable(marker) || !finite(marker.Start) || !finite(marker.End) || marker.Start < 0 || marker.End <= marker.Start || marker.Start >= duration {
			continue
		}
		for _, boundary := range []float64{marker.Start, marker.End} {
			if boundary <= 0 || boundary >= duration {
				continue
			}
			boundaries = append(boundaries, boundary)
			if len(boundaries) > maximumBoundaries {
				return ""
			}
		}
	}
	sort.Float64s(boundaries)
	intervals := make([][2]float64, 0, len(boundaries))
	for _, boundary := range boundaries {
		current := [2]float64{max(0, boundary-padding), boundary + padding}
		current[1] = min(duration, current[1])
		if len(intervals) > 0 && current[0] <= intervals[len(intervals)-1][1] {
			intervals[len(intervals)-1][1] = max(intervals[len(intervals)-1][1], current[1])
			continue
		}
		intervals = append(intervals, current)
	}
	values := make([]string, len(intervals))
	for index, interval := range intervals {
		values[index] = strconv.FormatFloat(interval[0], 'f', 3, 64) + "%" + strconv.FormatFloat(interval[1], 'f', 3, 64)
	}
	return strings.Join(values, ",")
}
