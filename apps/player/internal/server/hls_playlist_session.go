package server

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/hlsmanifest"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (manager *hlsManager) serveRecipe(writer http.ResponseWriter, request *http.Request, item library.Item, recipe hlsRecipe, name string) {
	localName, valid := localHLSFile(name)
	if !valid {
		localizedNotFound(writer, request)
		return
	}
	key := hlsRecipeKey(item.ID, recipe)
	start := 0
	duration := 0.0
	if filepath.Base(name) == "index.m3u8" {
		var validStart bool
		start, duration, validStart = manager.recipePlaylistStart(writer, request, item)
		if !validStart {
			return
		}
		if !manager.prepareRecipePlaylist(writer, request, item, recipe) {
			return
		}
	}
	if filepath.Base(name) == "index.m3u8" {
		writer.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		writer.Header().Set("Cache-Control", "no-store")
	}
	path := filepath.Join(manager.cache, key, localName)
	if filepath.Ext(name) == ".m3u8" && serveHLSPlaylistWithSession(writer, request, path, start, hlsPlaybackDuration(recipe, duration)) {
		return
	}
	if filepath.Ext(name) == ".m4s" {
		if !manager.waitForRecipeSegment(request, item, recipe, name, key, path) {
			localizedNotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "video/mp4")
	}
	//nolint:gosec // G703: filepath.Localize and hlsFile reject non-local and unknown paths above.
	http.ServeFile(writer, request, path)
}

func (manager *hlsManager) recipePlaylistStart(writer http.ResponseWriter, request *http.Request, item library.Item) (int, float64, bool) {
	start, err := requestedHLSStart(request)
	duration := manager.probe.duration(request.Context(), item)
	if err != nil || start > 0 && !validHLSOffset(float64(start), duration) {
		localizedError(writer, request, "resume position is invalid", http.StatusBadRequest)
		return 0, 0, false
	}
	return start, duration, true
}

func (manager *hlsManager) waitForRecipeSegment(request *http.Request, item library.Item, recipe hlsRecipe, name, key, path string) bool {
	manager.keepHLSAlive(key)
	segmentContext, cancel := context.WithTimeout(request.Context(), 30*time.Second)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := manager.prepareSegment(segmentContext, item, recipe, name); err != nil {
			slog.WarnContext(request.Context(), "HLS segment preparation failed", "diagnostic", "[PLAYBACK-HLS]", "request_id", requestActivityID(request.Context()), "error", hlsDiagnostic(err, item.Path))
		}
	}
	ready := waitForHLSFile(segmentContext, path)
	if !ready {
		slog.WarnContext(request.Context(), "HLS segment unavailable", "request_id", requestActivityID(request.Context()), "playback_session", requestPlaybackSession(request.Context()), "file", name, "mode", recipe.mode, "canceled", request.Context().Err() != nil)
	}
	cancel()
	return ready
}

func hlsSegmentNumber(name string) (int, bool) {
	if !strings.HasPrefix(name, "segment-") || !strings.HasSuffix(name, ".m4s") {
		return 0, false
	}
	value := strings.TrimSuffix(strings.TrimPrefix(name, "segment-"), ".m4s")
	segment, err := strconv.Atoi(value)
	return segment, err == nil && len(value) == 5 && segment >= 0 && segment <= 99_999
}

func hlsSegmentOffset(manifest []byte, target string, duration float64) (float64, bool) {
	if _, valid := hlsSegmentNumber(target); !valid {
		return 0, false
	}
	manifest = completeHLSVOD(manifest, duration)
	offset, segmentDuration, segments := 0.0, 0.0, 0
	for _, line := range strings.Split(string(manifest), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#EXTINF:") {
			value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
			segmentDuration, _ = strconv.ParseFloat(value, 64)
			if invalidHLSSegmentDuration(segmentDuration) {
				return 0, false
			}
			continue
		}
		if line == target && segmentDuration > 0 {
			return offset, true
		}
		if _, valid := hlsSegmentNumber(line); !(valid && segmentDuration > 0) {
			continue
		}
		segments++
		if segments > 100_000 {
			return 0, false
		}
		offset += segmentDuration
		segmentDuration = 0
	}
	return 0, false
}

func localHLSFile(name string) (string, bool) {
	localName, err := filepath.Localize(name)
	return localName, err == nil && hlsFile(name)
}

func serveHLSPlaylistWithSession(writer http.ResponseWriter, request *http.Request, path string, start int, duration float64) bool {
	playID := jellyfinPlaySessionQuery(request)
	if playID != "" && !validPlaybackSession(playID) {
		return false
	}
	//nolint:gosec // G703: serveRecipe constructs path from the HLS allowlist.
	manifest, err := os.ReadFile(path)
	if err != nil {
		localizedNotFound(writer, request)
		return true
	}
	if playID == "" {
		manifest = completeHLSVOD(manifest, duration)
	} else {
		manifest = hlsPlaylistWithSession(manifest, playID, jellyfinMediaQueryToken(request), start, duration)
	}
	if ticket, ok := request.Context().Value(castTicketKey{}).(string); ok {
		manifest = hlsPlaylistWithQuery(manifest, url.Values{"ticket": {ticket}})
	}
	//nolint:gosec // G705: the response is HLS, and all inserted query values are validated scalars.
	_, _ = writer.Write(manifest)
	return true
}

func hlsPlaylistWithSession(manifest []byte, playID, token string, start int, duration float64) []byte {
	query := url.Values{"playSessionId": {playID}}
	if token != "" {
		query.Set("api_key", token)
	}
	if start > 0 {
		query.Set("start", strconv.Itoa(start))
	}
	manifest = completeHLSVOD(manifest, duration)
	text := strings.Replace(string(manifest), "#EXT-X-PLAYLIST-TYPE:VOD\n", "#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-START:TIME-OFFSET="+strconv.Itoa(start)+",PRECISE=YES\n", 1)
	text = strings.Replace(text, "#EXT-X-PLAYLIST-TYPE:EVENT\n", "#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-START:TIME-OFFSET="+strconv.Itoa(start)+",PRECISE=YES\n", 1)
	return hlsPlaylistWithQuery([]byte(text), query)
}

func hlsPlaylistWithQuery(manifest []byte, query url.Values) []byte {
	lines := strings.Split(string(manifest), "\n")
	for index, line := range lines {
		if line != "" && !strings.HasPrefix(line, "#") {
			lines[index] = hlsURIWithQuery(line, query.Encode())
			continue
		}
		start := strings.Index(line, `URI="`)
		if start < 0 {
			continue
		}
		start += len(`URI="`)
		end := strings.IndexByte(line[start:], '"')
		if end >= 0 {
			lines[index] = line[:start] + hlsURIWithQuery(line[start:start+end], query.Encode()) + line[start+end:]
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

func completeHLSVOD(manifest []byte, duration float64) []byte {
	result, _ := hlsmanifest.CompleteVOD(manifest, duration, 2)
	return result
}

func invalidHLSSegmentDuration(duration float64) bool {
	return duration <= 0 || duration > 60 || math.IsNaN(duration) || math.IsInf(duration, 0)
}

func hlsPlaybackDuration(recipe hlsRecipe, duration float64) float64 {
	return playback.HLSPlaybackDuration(sharedHLSRecipe(recipe), duration)
}

func waitForHLSFile(ctx context.Context, path string) bool {
	return playback.WaitHLSReady(ctx, func() error {
		_, err := os.Stat(path)
		return err
	})
}

func requestedHLSStart(request *http.Request) (int, error) {
	values := request.URL.Query()["start"]
	if len(values) == 0 {
		return 0, nil
	}
	if len(values) != 1 || len(values[0]) == 0 || len(values[0]) > 9 {
		return 0, errors.New("resume position is invalid")
	}
	start, err := strconv.Atoi(values[0])
	if err != nil || start <= 0 || start > 7*24*60*60 {
		return 0, errors.New("resume position is invalid")
	}
	return start, nil
}

func hlsURIWithQuery(uri, query string) string {
	if strings.Contains(uri, "?") {
		return uri + "&" + query
	}
	return uri + "?" + query
}

func (manager *hlsManager) prepareRecipePlaylist(writer http.ResponseWriter, request *http.Request, item library.Item, recipe hlsRecipe) bool {
	prepareContext := context.WithValue(manager.ctx, requestActivityKey{}, &requestActivity{id: requestActivityID(request.Context()), playbackSession: requestPlaybackSession(request.Context())})
	prepareContext, cancel := context.WithTimeout(prepareContext, 30*time.Second)
	defer cancel()
	if err := manager.prepare(prepareContext, item, recipe); err != nil { //nolint:contextcheck // Playlist preparation uses the Server lifecycle so a disconnected request does not destroy shared output.
		slog.ErrorContext(request.Context(), "HLS playlist preparation failed", "diagnostic", "[PLAYBACK-HLS]", "request_id", requestActivityID(request.Context()), "mode", recipe.mode, "error", hlsDiagnostic(err, item.Path))
		status := http.StatusServiceUnavailable
		if errors.Is(err, playback.ErrHLSSource) {
			status = http.StatusBadRequest
		}
		localizedError(writer, request, err.Error(), status)
		return false
	}
	return true
}
