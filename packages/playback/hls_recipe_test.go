package playback

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestHLSRecipeRoundTripAndRoutes(t *testing.T) {
	t.Parallel()
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	plan := PlaybackPlan{Mode: "transcode", VideoCodec: "hevc", AudioIndex: 2, SubtitleSourceIndex: 7, SubtitleMode: "burn-in", SubtitleText: true, ColorMode: "tone-map-sdr", MaxBitrate: 4_000_000, Timeline: Timeline{Omitted: []Range{{Start: 10, End: 20}}}}
	recipe := RecipeFor(plan)
	recipe.DialogueBoost, recipe.NormalizeLoudness = true, true
	recipe.Offset = 30.1
	token := recipe.Token()
	parsed, err := ParseHLSRecipe(token, policy)
	if err != nil || !reflect.DeepEqual(parsed, recipe) {
		t.Fatalf("round trip = %#v, %v; token %q", parsed, err, token)
	}
	if got := HLSPlanURL("0123456789abcdef", plan); !strings.HasPrefix(got, "/hls/0123456789abcdef/p/t-a2-s7-text-t1-b4000000-chevc-k") {
		t.Fatalf("plan URL = %q", got)
	}
	planned, file, ok := PlannedHLSFile("p/"+token+"/720p/index.m3u8", policy)
	if !ok || file != "720p/index.m3u8" || !reflect.DeepEqual(planned, recipe) {
		t.Fatalf("planned route = %#v, %q, %v", planned, file, ok)
	}
	if _, file, ok := PlannedHLSFile("index.m3u8", policy); ok || file != "index.m3u8" {
		t.Fatalf("legacy route = %q, %v", file, ok)
	}
}

func TestParseHLSRecipeRejectsEveryInvalidBoundary(t *testing.T) {
	t.Parallel()
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	valid := "t-a0-s0-none-t0-b0"
	tests := []string{
		"", "x-a0-s0-none-t0-b0", "t-x0-s0-none-t0-b0", "t-a32-s0-none-t0-b0", "t-a0-s256-none-t0-b0",
		"t-a0-s0-unknown-t0-b0", "t-a0-s0-none-t2-b0", "t-a0-s0-none-t0-b100000001", "r-a0-s0-none-t0-b0-chevc",
		valid + "-cauto", valid + "-cunknown", valid + "-e0", valid + "-e4", valid + "-eno", "r-a0-s0-none-t0-b0-e1", valid + "-k", valid + "-k1_1", valid + "-k2_3.1_2",
		valid + "-k1_zzzzzzzzzz", valid + "-o99", valid + "-o604800100", valid + "-extra-extra-extra-extra",
	}
	for _, value := range tests {
		if _, err := ParseHLSRecipe(value, policy); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	if _, err := ParseHLSRecipe(valid, HLSRecipePolicy{}); err == nil {
		t.Fatal("accepted invalid recipe policy")
	}
	ranges := make([]string, 129)
	for index := range ranges {
		ranges[index] = "1_2"
	}
	if _, err := ParseHLSRecipe(valid+"-k"+strings.Join(ranges, "."), policy); err == nil {
		t.Fatal("accepted excessive automatic skip ranges")
	}
}

func TestHLSRecipeCacheAndTimelineHelpers(t *testing.T) { //nolint:cyclop // Related cache and timeline invariants share one table-driven contract test.
	t.Parallel()
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	recipe := HLSRecipe{Mode: "transcode", Codec: "h264", Audio: 0}
	if got := HLSRecipeKey("0123456789abcdef", recipe); got != "0123456789abcdef" {
		t.Fatalf("canonical cache key = %q", got)
	}
	recipe.SingleQuality = true
	if got := HLSRecipeKey("0123456789abcdef", recipe); !strings.HasSuffix(got, "-single") {
		t.Fatalf("single cache key = %q", got)
	}
	recipe = HLSRecipe{Mode: "transcode", Codec: "hevc", Audio: 1, Offset: 30, Omitted: []Range{{Start: 10, End: 20}, {Start: 40, End: 50}}}
	if got := HLSRecipeKey("0123456789abcdef", recipe); !strings.Contains(got, "-plan-") {
		t.Fatalf("planned key = %q", got)
	}
	if plain, effected := HLSRecipeKey("0123456789abcdef", HLSRecipe{Mode: "transcode", Codec: "h264"}), HLSRecipeKey("0123456789abcdef", HLSRecipe{Mode: "transcode", Codec: "h264", DialogueBoost: true}); plain == effected || !strings.Contains(effected, "-e1") {
		t.Fatalf("audio effect cache identity = %q, %q", plain, effected)
	}
	start, window := HLSWindowRecipe(recipe, 100)
	if start != 50 || window.Offset != 0 || len(window.Omitted) != 0 {
		t.Fatalf("window = start %v, %#v", start, window)
	}
	if start, unchanged := HLSWindowRecipe(HLSRecipe{Mode: "remux"}, 100); start != 0 || unchanged.Mode != "remux" {
		t.Fatal("zero offset recipe changed")
	}
	if !ValidHLSOffset(99, 100) || ValidHLSOffset(100, 100) || ValidHLSOffset(0, 0) {
		t.Fatal("offset validation changed")
	}
	timeline, err := TimelineFromPlaybackToken(recipe.Token(), policy)
	if err != nil || timeline.SourceDuration != math.MaxFloat64 || len(timeline.Omitted) != 2 {
		t.Fatalf("token timeline = %#v, %v", timeline, err)
	}
	if empty, err := TimelineFromPlaybackToken("", policy); err != nil || !reflect.DeepEqual(empty, Timeline{}) {
		t.Fatalf("empty timeline = %#v, %v", empty, err)
	}
	if _, err := TimelineFromPlaybackToken(HLSRecipe{Mode: "remux"}.Token(), policy); err == nil {
		t.Fatal("accepted token without omitted ranges")
	}
}

func TestHLSFilterAndBitrateArguments(t *testing.T) { //nolint:cyclop // Related FFmpeg argument invariants share one focused contract test.
	t.Parallel()
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	ranges := []Range{{Start: 1, End: 2}}
	recipe := HLSRecipe{Mode: "transcode", Burn: "text", Subtitle: 3, Omitted: ranges}
	mapping, video := ApplyBurnIn([]string{"-vf", "scale=640:-2", "-c:v", "libx264"}, `/media/a:b's.mkv`, recipe, policy)
	if len(mapping) != 4 || mapping[0] != "-map" || mapping[1] != "0:v:0" || mapping[2] != "-vf" || !strings.Contains(mapping[3], "subtitles=filename=") || !strings.Contains(mapping[3], "select=") || !reflect.DeepEqual(video, []string{"-c:v", "libx264"}) {
		t.Fatalf("text burn-in = %#v, %#v", mapping, video)
	}
	recipe.Burn = "image"
	mapping, _ = ApplyBurnIn([]string{"-vf", "scale=640:-2"}, "movie.mkv", recipe, policy)
	if mapping[0] != "-filter_complex" || !strings.Contains(mapping[1], "overlay") {
		t.Fatalf("image burn-in = %#v", mapping)
	}
	recipe.Burn = ""
	mapping, _ = ApplyBurnIn([]string{"-vf", "scale=640:-2"}, "movie.mkv", recipe, policy)
	if !strings.Contains(strings.Join(mapping, " "), "select=") {
		t.Fatalf("timeline-only filter = %#v", mapping)
	}
	plainMap, plainVideo := ApplyBurnIn([]string{"-c:v", "copy"}, "movie.mkv", HLSRecipe{}, policy)
	if !reflect.DeepEqual(plainMap, []string{"-map", "0:v:0"}) || !reflect.DeepEqual(plainVideo, []string{"-c:v", "copy"}) {
		t.Fatal("plain mapping changed")
	}
	if audio := AutomaticSkipAudioArguments(recipe, policy); len(audio) != 2 || !strings.Contains(audio[1], "aselect=") {
		t.Fatalf("audio filter = %#v", audio)
	}
	if AutomaticSkipAudioArguments(HLSRecipe{}, policy) != nil {
		t.Fatal("empty audio filter was not nil")
	}
	effects := AudioFilterArguments(HLSRecipe{DialogueBoost: true, NormalizeLoudness: true}, policy)
	if len(effects) != 2 || !strings.Contains(effects[1], "equalizer=f=1600") || !strings.Contains(effects[1], "dynaudnorm=") || strings.Contains(effects[1], "alimiter=") {
		t.Fatalf("combined audio effects = %#v", effects)
	}
	dialogue := AudioFilterArguments(HLSRecipe{DialogueBoost: true}, policy)
	if len(dialogue) != 2 || !strings.Contains(dialogue[1], "alimiter=") {
		t.Fatalf("dialogue limiter = %#v", dialogue)
	}
	zeroPolicy := policy
	zeroPolicy.StartPresentationAtZero = true
	if got := AutomaticSkipAudioArguments(recipe, zeroPolicy); !strings.Contains(got[1], "STARTPTS") {
		t.Fatalf("Subtitles timestamp policy = %#v", got)
	}
	if CappedVideoRate("2000k", "128k", 1_000_000) != "872k" || CappedVideoRate("500k", "128k", 0) != "500k" || CappedVideoRate("500k", "128k", 1_000_000) != "500k" || FFmpegSeconds(1.25) != "1.250" {
		t.Fatal("rate or timestamp formatting changed")
	}
}
