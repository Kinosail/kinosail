package server

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

var errPlaybackSeek = errors.New("invalid playback seek")

type playbackSeek struct {
	position *float64
	recipe   *hlsRecipe
}

// Query encoding is admitted before source lookup. The operation below checks
// the selected source duration, conversion policy, and cache certification.
func requestedPlaybackSeek(request *http.Request) (playbackSeek, error) {
	if len(request.URL.RawQuery) > 8192 {
		return playbackSeek{}, errPlaybackSeek
	}
	query, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		return playbackSeek{}, errPlaybackSeek
	}
	var seek playbackSeek
	if values, present := query["position"]; present {
		if len(values) != 1 || len(values[0]) > 32 || !utf8.ValidString(values[0]) {
			return seek, errPlaybackSeek
		}
		position, err := strconv.ParseFloat(values[0], 64)
		if err != nil || !validSeekPosition(position) || strconv.FormatFloat(position, 'f', -1, 64) != values[0] {
			return seek, errPlaybackSeek
		}
		seek.position = &position
	}
	if values, present := query["recipe"]; present {
		if seek.position == nil || len(values) != 1 || len(values[0]) > 2048 || !utf8.ValidString(values[0]) {
			return seek, errPlaybackSeek
		}
		recipe, err := playback.ParseHLSRecipe(values[0], hlsPolicy())
		if err != nil || recipe.Offset != 0 || recipe.Token() != values[0] ||
			(recipe.Mode != "remux" && recipe.Mode != "audio-transcode") || recipe.Burn != "" || recipe.Subtitle != 0 || len(recipe.Omitted) != 0 {
			return seek, errPlaybackSeek
		}
		local := localHLSRecipe(recipe)
		seek.recipe = &local
	}
	return seek, nil
}

func validSeekPosition(position float64) bool {
	return !math.IsNaN(position) && !math.IsInf(position, 0) && !math.Signbit(position) && position <= 604800 &&
		math.Abs(position*10-math.Round(position*10)) < 0.0000001
}

func savedSeekPosition(position, duration float64) float64 {
	if math.IsNaN(position) || math.IsInf(position, 0) || position < 0.1 || position >= duration || position > 604800 {
		return 0
	}
	return math.Floor(position*10) / 10
}

// A plan may copy a positive video seek only from an already certified exact
// key. Cold requests never start a packet scan to decide their delivery mode.
func (manager *hlsManager) exactSeekPlan(ctx context.Context, item library.Item, facts MediaFacts, client ClientCapabilities, policy ViewerPolicy, plan PlaybackPlan, recipe hlsRecipe, position float64) (PlaybackPlan, error) {
	if !validSeekPosition(position) || !validHLSOffset(position, facts.Duration) {
		return PlaybackPlan{}, errPlaybackSeek
	}
	if err := ctx.Err(); err != nil {
		return PlaybackPlan{}, err
	}
	if position == 0 || facts.Kind != "video" || !plan.Allowed || plan.Mode != "remux" && plan.Mode != "audio-transcode" {
		return plan, nil
	}
	intent := NetworkIntent{ForceTranscode: true, AudioIndex: &plan.AudioIndex, MaxBitrate: plan.MaxBitrate}
	if plan.SubtitleIndex >= 0 {
		intent.SubtitleIndex = &plan.SubtitleIndex
	}
	if !policy.AllowPlayback || !policy.AllowTranscode {
		return playbackWithAutomaticSkip(facts, client, policy, intent, nil, nil), nil
	}
	recipe.offset = position
	certified, err := manager.certifiedCopiedSeek(ctx, item, recipe)
	if err != nil || certified {
		return plan, err
	}
	converted := playbackWithAutomaticSkip(facts, client, policy, intent, nil, nil)
	if converted.Allowed {
		converted.Reason = "exact-seek-required"
	}
	return converted, nil
}

func (manager *hlsManager) certifiedCopiedSeek(ctx context.Context, item library.Item, recipe hlsRecipe) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return false, err
	}
	cache, err := os.OpenRoot(manager.cache)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errCopiedHLSIndex
	}
	defer cache.Close()
	name := hlsRecipeKey(item.ID, recipe)
	info, err := cache.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil || !info.IsDir() {
		return false, errCopiedHLSIndex
	}
	root, err := cache.OpenRoot(name)
	if err != nil {
		return false, errCopiedHLSIndex
	}
	defer root.Close()
	held, err := root.Stat(".")
	if err != nil || !os.SameFile(info, held) {
		return false, errCopiedHLSIndex
	}
	if _, err := root.Lstat(".copy-timeline"); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, errCopiedHLSIndex
	}
	directory := filepath.Join(manager.cache, name)
	timeline, err := manager.readCopiedHLSTimelineRoot(ctx, directory, options.Cache, root)
	if err != nil {
		return false, err
	}
	if err := manager.validateHLSPolicy(ctx, item, recipe, options.Cache); err != nil {
		return false, err
	}
	after, err := cache.Lstat(name)
	if err != nil || !os.SameFile(info, after) {
		return false, errCopiedHLSIndex
	}
	return timeline.Clock != nil && math.Abs(timeline.point(0)-recipe.offset) < 0.0000001, nil
}

func (manager *hlsManager) serveSharedHLSRecipe(writer http.ResponseWriter, request *http.Request, item library.Item, recipe playback.HLSRecipe, file string) {
	local := localHLSRecipe(recipe)
	if recipe.Offset > 0 && item.Kind == "video" && (recipe.Mode == "remux" || recipe.Mode == "audio-transcode") {
		resolved, err := playback.ResolveHLSSource(recipe, mediaFactsFor(item, manager.probe.facts(request.Context(), item)), item.Subtitles)
		if err != nil {
			hlsInvalidSeek(writer, request)
			return
		}
		local = localHLSRecipe(resolved)
		certified, err := manager.certifiedCopiedSeek(request.Context(), item, local)
		if err != nil || !certified {
			localizedError(writer, request, "compatible seek requires a new playback plan", http.StatusConflict)
			return
		}
	}
	manager.serveRecipe(writer, request, item, local, file)
}
