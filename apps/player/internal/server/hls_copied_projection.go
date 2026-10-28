package server

import (
	"bytes"
	"context"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/privatefile"
)

const (
	copiedProjectionHeader  = "#KINOSAIL-PROJECTION:"
	copiedProjectionMaximum = 8 << 20
)

func copiedProjectionPath(directory, rendition string) string {
	return filepath.Join(directory, ".projection-"+filepath.Base(rendition)+".m3u8")
}

func (manager *hlsManager) stableCopiedProjection(ctx context.Context, item library.Item, recipe hlsRecipe, key, name string, duration float64, projection func([]byte) []byte) func([]byte) []byte {
	if recipe.mode != "remux" && recipe.mode != "audio-transcode" || filepath.Base(name) != "index.m3u8" || !playback.QualityDirectory(filepath.Base(filepath.Dir(name))) {
		return projection
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return projection
	}
	directory, rendition, playable := filepath.Join(manager.cache, key), filepath.Dir(name), hlsPlaybackDuration(recipe, duration)
	return func(manifest []byte) []byte {
		growing := !playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") && !manager.copiedHLSTimelinePresent(directory) && ctx.Err() == nil
		if saved, found := readCopiedProjection(directory, rendition, options.Cache); growing && found {
			return saved
		}
		projected := projectHLSPlaylist(manifest, playable, projection)
		if growing && playback.PlaylistHas(projected, "#EXT-X-ENDLIST") {
			_ = writeAtomicFile(copiedProjectionPath(directory, rendition), append([]byte(copiedProjectionHeader+options.Cache+"\n"), projected...))
		}
		return projected
	}
}

func readCopiedProjection(directory, rendition, policy string) ([]byte, bool) {
	data, err := privatefile.Read(copiedProjectionPath(directory, rendition), copiedProjectionMaximum)
	header := []byte(copiedProjectionHeader + policy + "\n")
	if err != nil || !bytes.HasPrefix(data, header) {
		return nil, false
	}
	manifest := data[len(header):]
	if !bytes.HasPrefix(manifest, []byte("#EXTM3U\n")) || !playback.PlaylistHas(manifest, "#EXT-X-PLAYLIST-TYPE:VOD") || !playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") {
		return nil, false
	}
	return manifest, true
}
