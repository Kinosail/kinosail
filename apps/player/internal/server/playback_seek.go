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
		position, err := parseSeekPosition(values)
		if err != nil {
			return seek, errPlaybackSeek
		}
		seek.position = &position
	}
	if values, present := query["recipe"]; present {
		recipe, err := parsePriorSeekRecipe(values, seek.position)
		if err != nil {
			return seek, errPlaybackSeek
		}
		seek.recipe = &recipe
	}
	return seek, nil
}

func parseSeekPosition(values []string) (float64, error) {
	if len(values) != 1 || len(values[0]) > 32 || !utf8.ValidString(values[0]) {
		return 0, errPlaybackSeek
	}
	position, err := strconv.ParseFloat(values[0], 64)
	if err != nil || !validSeekPosition(position) || strconv.FormatFloat(position, 'f', -1, 64) != values[0] {
		return 0, errPlaybackSeek
	}
	return position, nil
}

func parsePriorSeekRecipe(values []string, position *float64) (hlsRecipe, error) {
	if position == nil || len(values) != 1 || len(values[0]) > 2048 || !utf8.ValidString(values[0]) {
		return hlsRecipe{}, errPlaybackSeek
	}
	recipe, err := playback.ParseHLSRecipe(values[0], hlsPolicy())
	if err != nil {
		return hlsRecipe{}, errPlaybackSeek
	}
	if !validPriorSeekRecipe(recipe, values[0]) {
		return hlsRecipe{}, errPlaybackSeek
	}
	return localHLSRecipe(recipe), nil
}

func validPriorSeekRecipe(recipe playback.HLSRecipe, token string) bool {
	return recipe.Offset == 0 && recipe.Token() == token && (recipe.Mode == "remux" || recipe.Mode == "audio-transcode") &&
		recipe.Burn == "" && recipe.Subtitle == 0 && len(recipe.Omitted) == 0
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
	if !copiedVideoSeek(facts, plan, position) {
		return plan, nil
	}
	if !policy.AllowPlayback || !policy.AllowTranscode {
		return seekConversionPlan(facts, client, policy, plan), nil
	}
	recipe.offset = position
	certified, err := manager.certifiedCopiedSeek(ctx, item, recipe)
	if err != nil || certified {
		return plan, err
	}
	converted := seekConversionPlan(facts, client, policy, plan)
	if converted.Allowed {
		converted.Reason = "exact-seek-required"
	}
	return converted, nil
}

func copiedVideoSeek(facts MediaFacts, plan PlaybackPlan, position float64) bool {
	return position != 0 && facts.Kind == "video" && plan.Allowed && (plan.Mode == "remux" || plan.Mode == "audio-transcode")
}

func seekConversionPlan(facts MediaFacts, client ClientCapabilities, policy ViewerPolicy, plan PlaybackPlan) PlaybackPlan {
	intent := NetworkIntent{ForceTranscode: true, AudioIndex: &plan.AudioIndex, MaxBitrate: plan.MaxBitrate}
	if plan.SubtitleIndex >= 0 {
		intent.SubtitleIndex = &plan.SubtitleIndex
	}
	return playbackWithAutomaticSkip(facts, client, policy, intent, nil, nil)
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
	root, info, err := openCopiedSeekGeneration(cache, name)
	if err != nil || root == nil {
		return false, err
	}
	defer root.Close()
	return manager.validateCopiedSeekGeneration(ctx, item, recipe, options.Cache, cache, root, info, name)
}

func openCopiedSeekGeneration(cache *os.Root, name string) (*os.Root, os.FileInfo, error) {
	info, err := cache.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil || !info.IsDir() {
		return nil, nil, errCopiedHLSIndex
	}
	root, err := cache.OpenRoot(name)
	if err != nil {
		return nil, nil, errCopiedHLSIndex
	}
	held, err := root.Stat(".")
	if err != nil || !os.SameFile(info, held) {
		_ = root.Close()
		return nil, nil, errCopiedHLSIndex
	}
	if _, err := root.Lstat(".copy-timeline"); errors.Is(err, os.ErrNotExist) {
		_ = root.Close()
		return nil, nil, nil
	} else if err != nil {
		_ = root.Close()
		return nil, nil, errCopiedHLSIndex
	}
	return root, info, nil
}

func (manager *hlsManager) validateCopiedSeekGeneration(ctx context.Context, item library.Item, recipe hlsRecipe, policy string, cache, root *os.Root, info os.FileInfo, name string) (bool, error) {
	directory := filepath.Join(manager.cache, name)
	timeline, err := manager.readCopiedHLSTimelineRoot(ctx, directory, policy, root)
	if err != nil {
		return false, err
	}
	if err := manager.validateHLSPolicy(ctx, item, recipe, policy); err != nil {
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
