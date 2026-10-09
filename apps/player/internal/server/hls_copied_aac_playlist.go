package server

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

// Certified manifests retain the same opened generation as init and media assets.
func (manager *hlsManager) serveCopiedAACPlaylist(writer http.ResponseWriter, request *http.Request, item library.Item, recipe hlsRecipe, name, key string, start int, duration float64) bool {
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return rejectCopiedAACPlaylist(writer, request)
	}
	directory := filepath.Join(manager.cache, key)
	if !copiedAACPolicyRequired(options.Cache) {
		if manager.copiedAACMarkerPresent(directory) {
			return rejectCopiedAACPlaylist(writer, request)
		}
		return false
	}
	held, err := manager.openCopiedAACGeneration(request.Context(), item, recipe, directory)
	if err != nil {
		return rejectCopiedAACPlaylist(writer, request)
	}
	defer held.close()
	manifest, err := held.playlist(name, start, duration, request)
	if err != nil {
		return rejectCopiedAACPlaylist(writer, request)
	}
	held.release()
	held.release = nil // Network transfer holds immutable bytes and roots, not the metadata lease.
	return writeHLSPlaylist(writer, request, manifest)
}

func (held *copiedAACGeneration) playlist(name string, start int, duration float64, request *http.Request) ([]byte, error) {
	if !held.current() {
		return nil, errCopiedHLSIndex
	}
	manifest, projection, err := held.playlistContent(name)
	if err != nil {
		return nil, err
	}
	rendered := hlsPlaylistSessionData(request, manifest, start, duration, projection)
	if rendered == nil || !held.current() {
		return nil, errCopiedHLSIndex
	}
	return rendered, nil
}

func (held *copiedAACGeneration) playlistContent(name string) ([]byte, func([]byte) []byte, error) {
	master, err := copiedHLSCacheFile(held.root, "index.m3u8", maximumCopiedHLSTimelineBytes)
	if err != nil || !copiedAACMasterAllowed(master, held.policy, held.certificate.Rendition) {
		return nil, nil, errCopiedHLSIndex
	}
	switch name {
	case "index.m3u8":
		return master, nil, nil
	case held.certificate.Rendition + "/index.m3u8":
		manifest, err := copiedHLSCacheFile(held.media, "index.m3u8", maximumCopiedHLSTimelineBytes)
		return manifest, held.projectPlaylist, err
	default:
		return nil, nil, errCopiedHLSIndex
	}
}

func (held *copiedAACGeneration) projectPlaylist(manifest []byte) []byte {
	result, valid := copiedHLSManifest(manifest, held.timeline)
	if !valid {
		return nil
	}
	return result
}

func copiedAACMasterAllowed(master []byte, policy, rendition string) bool {
	lines := strings.Split(string(master), "\n")
	if lines[0] != "#EXTM3U" {
		return false
	}
	bindings, renditions := 0, 0
	for _, line := range lines {
		if strings.HasPrefix(line, "#KINOSAIL-TRANSCODER:") {
			if line != "#KINOSAIL-TRANSCODER:"+policy {
				return false
			}
			bindings++
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line != rendition+"/index.m3u8" {
			return false
		}
		renditions++
	}
	return bindings == 1 && renditions == 1
}

func rejectCopiedAACPlaylist(writer http.ResponseWriter, request *http.Request) bool {
	slog.WarnContext(request.Context(), "HLS copied playlist rejected", "request_id", requestActivityID(request.Context()), "playback_session", requestPlaybackSession(request.Context()), "failure_class", "invalid-generation-or-manifest")
	localizedNotFound(writer, request)
	return true
}
