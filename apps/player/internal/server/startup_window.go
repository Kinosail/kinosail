package server

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

// Every new marker/asset operation is rooted at the configured cache directory.
// Recipe and manifest allowlists remain required; Root also prevents symlink escape.
func (manager *hlsManager) startupMarker(key string, prepared bool) {
	root, err := os.OpenRoot(manager.cache)
	if err != nil {
		return
	}
	defer root.Close()
	name := filepath.Join(key, ".startup")
	if prepared {
		_ = root.WriteFile(name, []byte("1"), 0o600)
	} else {
		_ = root.Remove(name)
	}
}

func (manager *hlsManager) missingStartupSegment(item library.Item, recipe hlsRecipe) string {
	key := hlsRecipeKey(item.ID, recipe)
	directory := filepath.Join(manager.cache, key)
	master, err := playback.ReadHLSPlaylist(filepath.Join(directory, "index.m3u8"))
	if err != nil {
		return ""
	}
	root, err := os.OpenRoot(manager.cache)
	if err != nil {
		return ""
	}
	defer root.Close()
	duration := hlsPlaybackDuration(recipe, manager.probe.duration(manager.ctx, item))
	projection := manager.copiedStartupProjection(item, recipe, directory)
	for _, rendition := range strings.Split(string(master), "\n") {
		if !hlsFile(rendition) || !strings.HasSuffix(rendition, "/index.m3u8") {
			continue
		}
		if name := missingStartupRendition(root, directory, key, rendition, duration, projection); name != "" {
			return name
		}
	}
	return ""
}

func missingStartupRendition(root *os.Root, directory, key, rendition string, duration float64, projection ...func([]byte) []byte) string {
	manifest, err := playback.ReadHLSPlaylist(filepath.Join(directory, rendition))
	if err != nil {
		return ""
	}
	segments, _ := startupWindowSegments(manifest, duration, projection...)
	for _, segment := range segments {
		name := filepath.Join(filepath.Dir(rendition), segment)
		if _, err := root.Stat(filepath.Join(key, name)); os.IsNotExist(err) {
			return name
		}
	}
	return ""
}

func (manager *hlsManager) startupWindowReady(item library.Item, recipe hlsRecipe, contexts ...context.Context) bool {
	ctx := manager.ctx
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	if err := manager.qualifyCopiedAAC(ctx, item, recipe, false); err != nil {
		return false
	}
	if manager.cache == "" || manager.settings == nil {
		return false
	}
	key := hlsRecipeKey(item.ID, recipe)
	directory := filepath.Join(manager.cache, key)
	options, err := manager.seekSettings(item, recipe, directory)
	if err != nil || !masterFresh(filepath.Join(directory, "index.m3u8"), item.Path, options.Cache) {
		return false
	}
	if manager.completeHEVCStartup(manager.ctx, item, recipe) &&
		!cacheFresh(filepath.Join(directory, "index.m3u8"), item.Path, options.Cache) {
		return false
	}
	master, err := playback.ReadHLSPlaylist(filepath.Join(directory, "index.m3u8"))
	if err != nil {
		return false
	}
	root, err := os.OpenRoot(manager.cache)
	if err != nil {
		return false
	}
	defer root.Close()
	duration := hlsPlaybackDuration(recipe, manager.probe.duration(manager.ctx, item))
	return startupMasterReady(root, directory, key, master, duration, manager.copiedStartupProjection(item, recipe, directory))
}

func startupMasterReady(root *os.Root, directory, key string, master []byte, duration float64, projection ...func([]byte) []byte) bool {
	count := 0
	for _, name := range strings.Split(string(master), "\n") {
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		if !hlsFile(name) || !strings.HasSuffix(name, "/index.m3u8") {
			return false
		}
		if !startupRenditionReady(root, directory, key, name, duration, projection...) {
			return false
		}
		count++
	}
	return count > 0
}

func startupAssetReady(root *os.Root, name string) bool {
	info, err := root.Stat(name)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func startupRenditionReady(root *os.Root, directory, key, playlist string, duration float64, projection ...func([]byte) []byte) bool {
	manifest, err := playback.ReadHLSPlaylist(filepath.Join(directory, playlist))
	if err != nil || !startupAssetReady(root, filepath.Join(key, filepath.Dir(playlist), "init.mp4")) {
		return false
	}
	segments, valid := startupWindowSegments(manifest, duration, projection...)
	if !valid {
		return false
	}
	for _, name := range segments {
		if !startupAssetReady(root, filepath.Join(key, filepath.Dir(playlist), name)) {
			return false
		}
	}
	return true
}

func startupWindowSegments(manifest []byte, playableDuration float64, projection ...func([]byte) []byte) ([]string, bool) {
	ordinary := len(projection) == 0 || projection[0] == nil
	if ordinary {
		segments, applicable, ready := remainingRoundedAACStartup(manifest, playableDuration)
		if applicable {
			return segments, ready
		}
	}
	manifest = projectHLSPlaylist(manifest, playableDuration, projection...)
	var segments []string
	duration, segmentDuration := 0.0, 0.0
	for _, name := range strings.Split(string(manifest), "\n") {
		if strings.HasPrefix(name, "#EXTINF:") {
			value, _, _ := strings.Cut(strings.TrimPrefix(name, "#EXTINF:"), ",")
			segmentDuration, _ = strconv.ParseFloat(value, 64)
			if invalidHLSSegmentDuration(segmentDuration) {
				return nil, false
			}
		}
		if _, valid := hlsSegmentNumber(name); !valid {
			continue
		}
		if segmentDuration == 0 {
			return nil, false
		}
		segments = append(segments, name)
		duration += segmentDuration
		segmentDuration = 0
		if duration >= 8 {
			ready := !ordinary || playableDuration <= 0 || duration <= playableDuration || playback.PlaylistHas(manifest, "#EXT-X-ENDLIST")
			return segments, ready
		}
	}
	return segments, len(segments) > 0 && playback.PlaylistHas(manifest, "#EXT-X-ENDLIST")
}
