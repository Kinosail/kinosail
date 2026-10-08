package server

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail/packages/isobmff"
	"github.com/MikeO7/kinosail/packages/playback"
)

func publishVariants(ctx context.Context, source, directory, transcoder string, qualities []PlaybackQuality, results <-chan error, expected int, independent bool) error {
	return playback.PublishVariantsObserved(ctx, source, directory, transcoder, qualities, results, expected, independent, writeAtomicFile, func() { hlsObservationFor(ctx).emit("media_ready", "") })
}

func masterFresh(playlist, source, transcoder string) bool {
	return playback.MasterFresh(playlist, source, transcoder)
}

func cacheFresh(playlist, source, transcoder string) bool {
	return playback.CacheFresh(playlist, source, transcoder)
}

func seekCacheFresh(directory, source, transcoder string) bool {
	return playback.SeekCacheFresh(directory, source, transcoder)
}

func hlsURIWithQuery(uri, query string) string {
	if strings.Contains(uri, "?") {
		return uri + "&" + query
	}
	return uri + "?" + query
}

func sourceVersion(path string) string          { return playback.SourceVersion(path) }
func finalizePlaylist(playlist string) error    { return playback.FinalizePlaylist(playlist) }
func hlsFile(name string) bool                  { return playback.HLSFile(name) }
func audioTrackIndex(value string) (int, error) { return playback.AudioTrackIndex(value) }

func (manager *hlsManager) initializationsReady(directory string) bool {
	names, err := manager.readHLSMasterRenditions(directory)
	if err != nil {
		return false
	}
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return false
	}
	defer root.Close()
	for _, name := range names {
		data, readErr := copiedHLSCacheFile(root, filepath.Join(filepath.Dir(name), "init.mp4"), isobmff.MaximumInitializationBytes)
		if readErr != nil {
			return false
		}
		if _, parseErr := isobmff.Parse(data); parseErr != nil {
			return false
		}
	}
	return true
}

// Bound the complete URI set before opening any advertised initialization.
// The generated adaptive ladder has at most five renditions.
func (manager *hlsManager) readHLSMasterRenditions(directory string) ([]string, error) {
	master, err := manager.readHLSRecipeManifest(directory)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, 5)
	seen := make(map[string]bool, 5)
	for _, name := range strings.Split(string(master), "\n") {
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		if !hlsFile(name) || !strings.HasSuffix(name, "/index.m3u8") || seen[name] || len(names) == 5 {
			return nil, errCopiedHLSIndex
		}
		seen[name] = true
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, errCopiedHLSIndex
	}
	return names, nil
}

func (manager *hlsManager) readHLSRecipeManifest(directory string) ([]byte, error) {
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	// Preserve the existing warming wait for a not-yet-published playlist.
	if _, err := root.Lstat("index.m3u8"); err != nil {
		return nil, err
	}
	return readHLSManifest(root)
}

func hlsURIWithQuery(uri, query string) string {
	if strings.Contains(uri, "?") {
		return uri + "&" + query
	}
	return uri + "?" + query
}
