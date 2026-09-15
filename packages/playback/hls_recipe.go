package playback

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

const maximumRecipeDurationMilliseconds = 7 * 24 * 60 * 60 * 1000

// HLSRecipe is the validated, cache-safe representation of one playback plan.
type HLSRecipe struct {
	Mode, Burn, Codec      string
	Audio, Subtitle        int
	ToneMap, SingleQuality bool
	MaxBitrate             int64
	Omitted                []Range
	Offset                 float64
	Width, Height          int
	SubtitleOrdinal        int
	SubtitlePath           string
	SubtitleTime           float64
	OutputTime             float64
}

// HLSRecipePolicy keeps the few legacy product differences outside the grammar.
type HLSRecipePolicy struct {
	MaxBitrate              int64
	OffsetStepMilliseconds  int64
	StartPresentationAtZero bool
}

func RecipeFor(plan PlaybackPlan) HLSRecipe {
	recipe := HLSRecipe{Mode: plan.Mode, Codec: plan.VideoCodec, Audio: plan.AudioIndex, Subtitle: plan.SubtitleSourceIndex, ToneMap: plan.ColorMode == "tone-map-sdr", MaxBitrate: plan.MaxBitrate, Omitted: append([]Range(nil), plan.Timeline.Omitted...)}
	if plan.SubtitleMode == "burn-in" {
		switch {
		case plan.SubtitleExternal:
			recipe.Burn, recipe.Subtitle = "external", plan.SubtitleExternalIndex
		case plan.SubtitleText:
			recipe.Burn = "text"
		default:
			recipe.Burn = "image"
		}
	}
	if recipe.Mode == "transcode" {
		recipe.Width, recipe.Height = plan.Width, plan.Height
	}
	if recipe.Burn == "" {
		recipe.Subtitle = 0
	}
	return recipe
}

func (recipe HLSRecipe) Token() string {
	mode := map[string]string{"remux": "r", "audio-transcode": "a", "transcode": "t"}[recipe.Mode]
	burn := recipe.Burn
	if burn == "" {
		burn = "none"
	}
	token := fmt.Sprintf("%s-a%d-s%d-%s-t%d-b%d", mode, recipe.Audio, recipe.Subtitle, burn, boolInt(recipe.ToneMap), recipe.MaxBitrate)
	if recipe.Mode == "transcode" && transcodepolicy.NormalizeCodec(recipe.Codec) != "h264" {
		token += "-c" + recipe.Codec
	}
	if recipe.Width > 0 {
		token += fmt.Sprintf("-z%dx%d", recipe.Width, recipe.Height)
	}
	if len(recipe.Omitted) > 0 {
		ranges := make([]string, 0, len(recipe.Omitted))
		for _, value := range recipe.Omitted {
			ranges = append(ranges, strconv.FormatInt(int64(math.Round(value.Start*1000)), 36)+"_"+strconv.FormatInt(int64(math.Round(value.End*1000)), 36))
		}
		token += "-k" + strings.Join(ranges, ".")
	}
	if recipe.Offset > 0 {
		token += "-o" + strconv.FormatInt(int64(math.Round(recipe.Offset*1000)), 10)
	}
	return token
}

func ParseHLSRecipe(value string, policy HLSRecipePolicy) (HLSRecipe, error) { //nolint:cyclop,gocognit // Every bounded token field is validated before use.
	if policy.MaxBitrate <= 0 || policy.OffsetStepMilliseconds <= 0 {
		return HLSRecipe{}, errors.New("playback recipe policy is invalid")
	}
	if len(value) > 8192 {
		return HLSRecipe{}, errors.New("playback recipe is too large")
	}
	parts := strings.Split(value, "-")
	if len(parts) < 6 || len(parts) > 10 || len(value) > 8192 {
		return HLSRecipe{}, errors.New("playback recipe is invalid")
	}
	parts, offset, err := parseRecipeOffset(parts, policy.OffsetStepMilliseconds)
	if err != nil {
		return HLSRecipe{}, errors.New("playback recipe is invalid")
	}
	parts, codec, err := parseRecipeCodec(parts)
	if err != nil {
		return HLSRecipe{}, err
	}
	parts, width, height, err := parseRecipeSize(parts)
	if err != nil || len(parts) != 6 && len(parts) != 7 {
		return HLSRecipe{}, errors.New("playback recipe is invalid")
	}
	recipe, err := parseRecipeFields(parts, codec, policy.MaxBitrate)
	if err != nil {
		return HLSRecipe{}, errors.New("playback recipe is invalid")
	}
	omitted, err := parseOmittedRanges(parts)
	if err != nil {
		return HLSRecipe{}, errors.New("playback recipe is invalid")
	}
	recipe.Omitted, recipe.Offset = omitted, float64(offset)/1000
	recipe.Width, recipe.Height = width, height
	if recipe.Mode != "transcode" && (recipe.Burn != "" || recipe.ToneMap || width != 0) {
		return HLSRecipe{}, errors.New("conversion fields require video conversion")
	}
	return recipe, nil
}

func parseRecipeOffset(parts []string, step int64) ([]string, int64, error) {
	if !strings.HasPrefix(parts[len(parts)-1], "o") {
		return parts, 0, nil
	}
	offset, err := strconv.ParseInt(strings.TrimPrefix(parts[len(parts)-1], "o"), 10, 64)
	if err != nil || offset <= 0 || offset > maximumRecipeDurationMilliseconds || offset%step != 0 {
		return nil, 0, errors.New("invalid offset")
	}
	return parts[:len(parts)-1], offset, nil
}

func parseRecipeCodec(parts []string) ([]string, string, error) {
	if len(parts) <= 6 || !strings.HasPrefix(parts[6], "c") {
		return parts, "", nil
	}
	codec := strings.TrimPrefix(parts[6], "c")
	if codec == "" || codec == "auto" || !transcodepolicy.ValidCodec(codec) {
		return nil, "", errors.New("invalid codec")
	}
	return append(parts[:6], parts[7:]...), codec, nil
}

func parseRecipeFields(parts []string, codec string, maximumBitrate int64) (HLSRecipe, error) { //nolint:cyclop // Compact recipe-field validation remains below the repository quality ceiling.
	mode := map[string]string{"r": "remux", "a": "audio-transcode", "t": "transcode"}[parts[0]]
	if mode == "" || codec != "" && mode != "transcode" {
		return HLSRecipe{}, errors.New("playback recipe is invalid")
	}
	audio, audioErr := prefixedInt(parts[1], "a")
	subtitle, subtitleErr := prefixedInt(parts[2], "s")
	toneMap, toneErr := prefixedInt(parts[4], "t")
	bitrate, bitrateErr := prefixedInt64(parts[5], "b")
	burn := parts[3]
	if audioErr != nil || subtitleErr != nil || toneErr != nil || bitrateErr != nil || audio < 0 || audio > 31 || subtitle < 0 || subtitle > 255 || !oneOf(burn, "none", "text", "image", "external") || toneMap < 0 || toneMap > 1 || bitrate < 0 || bitrate > maximumBitrate {
		return HLSRecipe{}, errors.New("invalid fields")
	}
	if burn == "none" {
		burn = ""
	}
	return HLSRecipe{Mode: mode, Burn: burn, Codec: codec, Audio: audio, Subtitle: subtitle, ToneMap: toneMap == 1, MaxBitrate: bitrate}, nil
}

func prefixedInt(value, prefix string) (int, error) {
	if !strings.HasPrefix(value, prefix) {
		return 0, errors.New("prefix is invalid")
	}
	return strconv.Atoi(strings.TrimPrefix(value, prefix))
}

func prefixedInt64(value, prefix string) (int64, error) {
	if !strings.HasPrefix(value, prefix) {
		return 0, errors.New("prefix is invalid")
	}
	return strconv.ParseInt(strings.TrimPrefix(value, prefix), 10, 64)
}

func parseOmittedRanges(parts []string) ([]Range, error) { //nolint:cyclop // Cardinality, ordering, and bounds form one validation pass.
	if len(parts) == 6 {
		return nil, nil
	}
	encoded := strings.TrimPrefix(parts[6], "k")
	if encoded == parts[6] || encoded == "" {
		return nil, errors.New("automatic skip ranges are invalid")
	}
	values := strings.Split(encoded, ".")
	if len(values) > 128 {
		return nil, errors.New("too many automatic skip ranges")
	}
	ranges := make([]Range, 0, len(values))
	for _, value := range values {
		startText, endText, found := strings.Cut(value, "_")
		start, startErr := strconv.ParseInt(startText, 36, 64)
		end, endErr := strconv.ParseInt(endText, 36, 64)
		if !found || startErr != nil || endErr != nil || start < 0 || end <= start || end > maximumRecipeDurationMilliseconds || len(ranges) > 0 && float64(start)/1000 < ranges[len(ranges)-1].End {
			return nil, errors.New("automatic skip range is invalid")
		}
		ranges = append(ranges, Range{Start: float64(start) / 1000, End: float64(end) / 1000})
	}
	return ranges, nil
}

func TimelineFromPlaybackToken(value string, policy HLSRecipePolicy) (Timeline, error) {
	if value == "" {
		return Timeline{}, nil
	}
	recipe, err := ParseHLSRecipe(value, policy)
	if err != nil || len(recipe.Omitted) == 0 {
		return Timeline{}, errors.New("playback token is invalid")
	}
	return Timeline{SourceDuration: math.MaxFloat64, Duration: math.MaxFloat64, Omitted: recipe.Omitted}, nil
}

func HLSPlanURL(id string, plan PlaybackPlan) string {
	return "/hls/" + id + "/p/" + RecipeFor(plan).Token() + "/index.m3u8"
}

func PlannedHLSFile(name string, policy HLSRecipePolicy) (HLSRecipe, string, bool) {
	parts := strings.Split(name, "/")
	if len(parts) < 3 || parts[0] != "p" {
		return HLSRecipe{}, name, false
	}
	recipe, err := ParseHLSRecipe(parts[1], policy)
	return recipe, strings.Join(parts[2:], "/"), err == nil
}

func HLSRecipeKey(id string, recipe HLSRecipe) string { //nolint:cyclop // Every non-default recipe field must prevent reuse of the legacy cache identity.
	if recipe.SingleQuality {
		return id + "-plan-" + recipe.Token() + "-single"
	}
	if recipe.Mode == "transcode" && transcodepolicy.NormalizeCodec(recipe.Codec) == "h264" && recipe.Burn == "" && !recipe.ToneMap && recipe.MaxBitrate == 0 && recipe.Width == 0 && recipe.Height == 0 && len(recipe.Omitted) == 0 && recipe.Offset == 0 {
		return HLSCacheKey(id, recipe.Audio)
	}
	return id + "-plan-" + recipe.Token()
}

func ValidHLSOffset(offset, duration float64) bool { return duration > 0 && offset < duration }

func HLSWindowRecipe(recipe HLSRecipe, duration float64) (float64, HLSRecipe) {
	if recipe.Offset == 0 {
		return 0, recipe
	}
	timeline := Timeline{SourceDuration: duration, Duration: duration, Omitted: recipe.Omitted}
	for _, omitted := range recipe.Omitted {
		timeline.Duration -= omitted.End - omitted.Start
	}
	start := timeline.SourceTime(recipe.Offset)
	window := recipe
	window.Offset, window.Omitted = 0, nil
	for _, omitted := range recipe.Omitted {
		if omitted.End > start {
			window.Omitted = append(window.Omitted, Range{Start: max(0, omitted.Start-start), End: omitted.End - start})
		}
	}
	return start, window
}

func CappedVideoRate(videoRate, audioRate string, maximum int64) string {
	if maximum == 0 {
		return videoRate
	}
	video, _ := strconv.ParseInt(strings.TrimSuffix(videoRate, "k"), 10, 64)
	audio, _ := strconv.ParseInt(strings.TrimSuffix(audioRate, "k"), 10, 64)
	available := maximum/1000 - audio
	if available > 0 && available < video {
		return strconv.FormatInt(available, 10) + "k"
	}
	return videoRate
}

func parseRecipeSize(parts []string) ([]string, int, int, error) {
	if len(parts) <= 6 || !strings.HasPrefix(parts[6], "z") {
		return parts, 0, 0, nil
	}
	w, h, ok := strings.Cut(strings.TrimPrefix(parts[6], "z"), "x")
	width, e1 := strconv.Atoi(w)
	height, e2 := strconv.Atoi(h)
	if !ok || e1 != nil || e2 != nil || !validHLSSize(width, height) {
		return nil, 0, 0, errors.New("invalid output dimensions")
	}
	return append(parts[:6], parts[7:]...), width, height, nil
}
