package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail/packages/playback"
)

func publishCompletedCopiedHLS(ctx context.Context, source, directory, policy string, qualities []PlaybackQuality, results <-chan error, expected int, plan *copiedHLSPlan) error {
	if expected != 1 || len(qualities) != 1 {
		return errCopiedHLSPlan
	}
	if err := completedCopiedResult(ctx, results); err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := validateCompletedCopiedRendition(root, qualities[0].Label); err != nil {
		return err
	}
	if err := playback.ValidateHLSSource(source, policy); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return playback.WriteMaster(filepath.Join(directory, "index.m3u8"), policy, qualities, false, plan.bindMaster)
}

func completedCopiedResult(ctx context.Context, results <-chan error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err, open := <-results:
		if err != nil {
			return err
		}
		if !open {
			return errCopiedHLSPlan
		}
	}
	return nil
}

func validateCompletedCopiedRendition(root *os.Root, directory string) error {
	manifest, err := readHLSGenerationFile(root, filepath.Join(directory, "index.m3u8"), maxHLSPlaylistBytes)
	if err != nil {
		return err
	}
	if !playback.PlaylistHas(manifest, "#EXT-X-PLAYLIST-TYPE:EVENT") || !playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") {
		return errCopiedHLSPlan
	}
	if _, err := hlsPlaylistWithSession(manifest, ""); err != nil {
		return errCopiedHLSPlan
	}
	return validateCompletedCopiedAssets(root, directory, manifest)
}

func validateCompletedCopiedAssets(root *os.Root, directory string, manifest []byte) error {
	initialized, segments := false, 0
	for _, line := range strings.Split(string(manifest), "\n") {
		name := line
		switch {
		case line == `#EXT-X-MAP:URI="init.mp4"`:
			name, initialized = "init.mp4", true
		case line != "" && !strings.HasPrefix(line, "#"):
			segments++
		default:
			continue
		}
		if !completedCopiedAsset(root, directory, name) {
			return errCopiedHLSPlan
		}
	}
	if !initialized || segments == 0 {
		return errCopiedHLSPlan
	}
	return nil
}

func completedCopiedAsset(root *os.Root, directory, name string) bool {
	if !variantFile(name) {
		return false
	}
	info, err := root.Lstat(filepath.Join(directory, name))
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}
