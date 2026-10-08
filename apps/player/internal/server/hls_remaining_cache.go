package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/isobmff"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

type remainingAACAsset struct {
	file    *os.File
	info    os.FileInfo
	content io.ReadSeeker
}

func (manager *hlsManager) serveRemainingAACFile(writer http.ResponseWriter, request *http.Request, item library.Item, recipe hlsRecipe, name, key string) bool {
	if !remainingAACOriginRecipe(item, recipe) || filepath.Ext(name) != ".m4s" && filepath.Base(name) != "init.mp4" {
		return false
	}
	asset, err := manager.openRemainingAACAsset(request.Context(), item, recipe, name)
	if err != nil {
		slog.WarnContext(request.Context(), "HLS cache asset rejected", "diagnostic", "[PLAYBACK-HLS]", "request_id", requestActivityID(request.Context()), "playback_session", requestPlaybackSession(request.Context()), "mode", recipe.mode, "error", hlsDiagnostic(err, item.Path))
		localizedNotFound(writer, request)
		return true
	}
	defer asset.file.Close()
	if filepath.Ext(name) == ".m4s" {
		writer.Header().Set("Content-Type", "video/mp4")
	}
	manager.adoptRecipeFile(request, key, filepath.Join(manager.cache, key, name))
	http.ServeContent(writer, request, filepath.Base(name), asset.info.ModTime(), asset.content)
	return true
}

func (manager *hlsManager) openRemainingAACAsset(ctx context.Context, item library.Item, recipe hlsRecipe, name string) (*remainingAACAsset, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !hlsFile(name) || filepath.Dir(name) != "audio" {
		return nil, errCopiedHLSIndex
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return nil, err
	}
	root, err := manager.openCopiedHLSRoot(filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe)))
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	defer root.Close()
	if err := remainingAACReadyRoot(root, options.Cache); err != nil {
		return nil, err
	}
	if err := remainingAACManifestAsset(root, name, manager.probe.duration(ctx, item)); err != nil {
		return nil, err
	}
	file, info, err := copiedHLSOpenFile(root, name, 64<<20)
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	asset := &remainingAACAsset{file: file, info: info, content: io.NewSectionReader(file, 0, info.Size())}
	if err := manager.admitRemainingAACAsset(ctx, item, recipe, root, name, options.Cache, asset); err != nil {
		_ = file.Close()
		return nil, err
	}
	return asset, nil
}

func (manager *hlsManager) admitRemainingAACAsset(ctx context.Context, item library.Item, recipe hlsRecipe, root *os.Root, name, policy string, asset *remainingAACAsset) error {
	if filepath.Base(name) == "init.mp4" {
		if err := asset.retainInitialization(); err != nil {
			return err
		}
	}
	if err := manager.validateHLSPolicy(ctx, item, recipe, policy); err != nil {
		return err
	}
	after, err := root.Lstat(name)
	if err != nil || !sameCopiedHLSFile(asset.info, after) {
		return errCopiedHLSIndex
	}
	return nil
}

func (asset *remainingAACAsset) retainInitialization() error {
	data, err := io.ReadAll(io.LimitReader(asset.file, isobmff.MaximumInitializationBytes+1))
	if err != nil || int64(len(data)) != asset.info.Size() {
		return errCopiedHLSIndex
	}
	initialization, err := isobmff.Parse(data)
	if err != nil || len(initialization.Tracks) != 1 || initialization.Codecs() != "mp4a.40.2" {
		return errCopiedHLSIndex
	}
	asset.content = bytes.NewReader(data)
	return nil
}

func remainingAACOriginRecipe(item library.Item, recipe hlsRecipe) bool {
	return item.Kind == "audio" && remainingPlainAudio(recipe) && recipe.offset == 0 && recipe.outputTime == 0 &&
		(recipe.maxBitrate == 0 || recipe.maxBitrate >= 192000)
}

func (manager *hlsManager) remainingAACCacheAsset(ctx context.Context, item library.Item, recipe hlsRecipe, name string) error {
	if !remainingAACOriginRecipe(item, recipe) {
		return nil
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return err
	}
	directory := filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe))
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer root.Close()
	if err := remainingAACReadyRoot(root, options.Cache); err != nil {
		return err
	}
	if err := remainingAACManifestAsset(root, name, manager.probe.duration(ctx, item)); err != nil {
		return err
	}
	if err := remainingAACExistingAsset(root, name); err != nil {
		return err
	}
	return manager.validateHLSPolicy(ctx, item, recipe, options.Cache)
}

func remainingAACReadyRoot(root *os.Root, policy string) error {
	if err := remainingAACCacheIdentity(root, policy); err != nil {
		return err
	}
	if err := remainingAACValidateInitialization(root); err != nil {
		return err
	}
	return remainingAACExistingAsset(root, "audio/segment-00000.m4s")
}

func remainingAACCacheIdentity(root *os.Root, policy string) error {
	binding, err := copiedHLSCacheFile(root, ".source", 16<<10)
	if err != nil || string(binding) != policy {
		return errHLSIdentityChanged
	}
	master, err := copiedHLSCacheFile(root, "index.m3u8", 1<<20)
	if err != nil || !remainingAACMasterPolicy(master, policy) {
		return errHLSIdentityChanged
	}
	renditions := 0
	for _, line := range strings.Split(string(master), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line != "audio/index.m3u8" {
			return errCopiedHLSIndex
		}
		renditions++
	}
	if renditions != 1 {
		return errCopiedHLSIndex
	}
	return nil
}

func remainingAACMasterPolicy(master []byte, policy string) bool {
	return playback.PlaylistHas(master, "#KINOSAIL-TRANSCODER:"+policy) && strings.Contains(string(master), "#EXT-X-STREAM-INF:")
}

func remainingAACValidateInitialization(root *os.Root) error {
	data, err := copiedHLSCacheFile(root, "audio/init.mp4", isobmff.MaximumInitializationBytes)
	if err != nil {
		return errCopiedHLSIndex
	}
	initialization, err := isobmff.Parse(data)
	if err != nil || len(initialization.Tracks) != 1 || initialization.Codecs() != "mp4a.40.2" {
		return errCopiedHLSIndex
	}
	return nil
}

func remainingAACExistingAsset(root *os.Root, name string) error {
	if !hlsFile(name) || filepath.Dir(name) != "audio" {
		return errCopiedHLSIndex
	}
	if _, err := root.Lstat(name); errors.Is(err, os.ErrNotExist) {
		return nil // Projected future fragments continue through ordinary refill.
	}
	file, info, err := copiedHLSOpenFile(root, name, 64<<20)
	if err != nil {
		return errCopiedHLSIndex
	}
	_ = file.Close()
	after, err := root.Lstat(name)
	if err != nil || !sameCopiedHLSFile(info, after) {
		return errCopiedHLSIndex
	}
	return nil
}

func remainingAACManifestAsset(root *os.Root, name string, duration float64) error {
	manifest, err := remainingAACManifestRead(root)
	if err != nil || !remainingAACManifestValid(manifest) {
		return errCopiedHLSIndex
	}
	if filepath.Base(name) == "init.mp4" {
		return nil
	}
	_, valid := hlsSegmentOffset(manifest, filepath.Base(name), duration)
	if !valid {
		return errCopiedHLSIndex
	}
	return nil
}

func remainingAACManifestRead(root *os.Root) ([]byte, error) {
	file, before, err := copiedHLSOpenFile(root, "audio/index.m3u8", 1<<20)
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	defer file.Close()
	manifest, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	after, statErr := file.Stat()
	current, pathErr := root.Lstat("audio/index.m3u8")
	if err != nil || statErr != nil || pathErr != nil || int64(len(manifest)) != before.Size() ||
		!sameCopiedHLSFile(before, after) || !current.Mode().IsRegular() || current.Size() <= 0 || current.Size() > 1<<20 {
		return nil, errCopiedHLSIndex
	}
	return manifest, nil
}

type remainingAACManifestState struct {
	next, maps int
	pending    float64
	ended      bool
}

func remainingAACManifestValid(manifest []byte) bool {
	if !playback.PlaylistHas(manifest, "#EXTM3U") || !playback.PlaylistHas(manifest, "#EXT-X-MEDIA-SEQUENCE:0") {
		return false
	}
	state := remainingAACManifestState{}
	for _, line := range strings.Split(string(manifest), "\n") {
		if !state.line(strings.TrimSpace(line)) {
			return false
		}
	}
	return state.next > 0 && state.pending == 0 && state.maps == 1
}

func (state *remainingAACManifestState) line(line string) bool {
	switch {
	case strings.HasPrefix(line, "#EXTINF:"):
		return state.duration(line)
	case strings.HasPrefix(line, "#EXT-X-MAP:"):
		state.maps++
		return line == `#EXT-X-MAP:URI="init.mp4"`
	case strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:"):
		return line == "#EXT-X-MEDIA-SEQUENCE:0"
	case line == "#EXT-X-ENDLIST":
		state.ended = true
		return state.pending == 0
	case line == "" || strings.HasPrefix(line, "#"):
		return true
	default:
		return state.segment(line)
	}
}

func (state *remainingAACManifestState) duration(line string) bool {
	if state.pending != 0 || state.ended {
		return false
	}
	value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
	length, err := strconv.ParseFloat(value, 64)
	if err != nil || invalidHLSSegmentDuration(length) {
		return false
	}
	state.pending = length
	return true
}

func (state *remainingAACManifestState) segment(line string) bool {
	number, valid := hlsSegmentNumber(line)
	if !valid || number != state.next || state.pending == 0 || state.ended {
		return false
	}
	state.next++
	state.pending = 0
	return true
}
