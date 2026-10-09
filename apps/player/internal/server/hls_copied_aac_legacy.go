package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/library"
)

// Legacy reads keep the full base contract without changing producer eligibility.
func (manager *hlsManager) openCopiedHLSLegacyGeneration(parent context.Context, item library.Item, recipe hlsRecipe, directory string) (*copiedAACGeneration, error) {
	base, before, err := manager.copiedAACQualificationSource(parent, item, recipe)
	if err != nil || !copiedAACRecipeSupported(item, recipe) {
		return nil, errCopiedHLSIndex
	}
	if _, err := manager.copiedAACSettings(item, recipe, base); err != nil {
		return nil, errCopiedHLSIndex
	}
	ctx, release, err := manager.copiedHLSClockAdmission(parent)
	if err != nil {
		return nil, err
	}
	value := &copiedAACGeneration{
		manager:   manager,
		ctx:       ctx,
		item:      item,
		recipe:    recipe,
		directory: directory,
		policy:    base.Cache,
		release:   release,
		legacy:    &copiedHLSLegacyRead{sourceInfo: before, entries: make(map[string]os.FileInfo)},
	}
	value.legacy.source, _, err = openCopiedHLSSourceAudio(ctx, item.Path, before)
	if err == nil {
		err = value.open(ctx)
	}
	if err != nil {
		value.close()
		return nil, errCopiedHLSIndex
	}
	return value, nil
}

// Presence with an unreadable or mismatched binding is a handled rejection,
// never permission to enter preparation and replace the existing generation.
func (manager *hlsManager) copiedHLSLegacyBinding(item library.Item, recipe hlsRecipe, directory string) (bool, string) {
	root, err := manager.openCopiedHLSRoot(directory)
	if os.IsNotExist(err) {
		return false, ""
	}
	if err != nil {
		return true, "invalid-legacy-generation"
	}
	defer root.Close()
	present, err := copiedHLSLegacyIndexPresent(root)
	if err != nil {
		return true, "invalid-legacy-generation"
	}
	if !present {
		return false, ""
	}
	base, err := manager.baseHLSSettings(item, recipe)
	if err != nil {
		return true, "invalid-source-binding"
	}
	binding, err := copiedHLSCacheFile(root, ".source", 16<<10)
	if err != nil {
		return true, "invalid-source-binding"
	}
	if copiedAACPolicyRequired(string(binding)) {
		return false, "" // Strict Version2 qualification and admission stay unchanged.
	}
	if string(binding) != base.Cache {
		return true, "invalid-source-binding"
	}
	return true, ""
}

func copiedHLSLegacyIndexPresent(root *os.Root) (bool, error) {
	for _, name := range []string{".copy-timeline", ".copy-clock"} {
		_, err := root.Lstat(name)
		if err == nil {
			return true, nil
		}
		if !os.IsNotExist(err) {
			return false, errCopiedHLSIndex
		}
	}
	return false, nil
}

func (manager *hlsManager) serveCopiedHLSLegacy(writer http.ResponseWriter, request *http.Request, item library.Item, recipe hlsRecipe, name string) bool {
	if (request.Method != http.MethodGet && request.Method != http.MethodHead) || !copiedAACRecipeSupported(item, recipe) {
		return false
	}
	directory := filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe))
	handled, failure := manager.copiedHLSLegacyBinding(item, recipe, directory)
	if !handled {
		return false
	}
	if failure != "" {
		return rejectCopiedAACPlaylistClass(writer, request, failure)
	}
	held, err := manager.openCopiedHLSLegacyGeneration(request.Context(), item, recipe, directory)
	if err != nil {
		return rejectCopiedAACPlaylistClass(writer, request, "invalid-legacy-generation")
	}
	defer held.close()
	if filepath.Ext(name) == ".m3u8" {
		return held.serveLegacyPlaylist(writer, request, name)
	}
	return held.serveLegacyAsset(writer, request, name)
}

func (held *copiedAACGeneration) serveLegacyPlaylist(writer http.ResponseWriter, request *http.Request, name string) bool {
	start, err := requestedHLSStart(request)
	if err != nil || start > 0 && !validHLSOffset(float64(start), held.timeline.End) {
		warnCopiedAACPlaylist(request, "invalid-start")
		localizedError(writer, request, "resume position is invalid", http.StatusBadRequest)
		return true
	}
	manifest, err := held.playlist(name, start, hlsPlaybackDuration(held.recipe, held.timeline.End), request)
	if err != nil {
		return rejectCopiedAACPlaylistClass(writer, request, "invalid-legacy-generation")
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Kinosail-Startup-Cache", "warm")
	held.release()
	held.release = nil
	return writeHLSPlaylist(writer, request, manifest)
}

func (held *copiedAACGeneration) serveLegacyAsset(writer http.ResponseWriter, request *http.Request, name string) bool {
	file, info, content, err := held.openAsset(name)
	if err != nil {
		return rejectCopiedAACPlaylistClass(writer, request, "invalid-legacy-generation")
	}
	defer file.Close()
	writer.Header().Set("Content-Type", "video/mp4")
	held.release()
	held.release = nil // Transfer owns the opened asset and no metadata/probe lease.
	http.ServeContent(writer, request, filepath.Base(name), info.ModTime(), content)
	return true
}
