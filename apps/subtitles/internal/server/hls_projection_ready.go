package server

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/MikeO7/kinosail/packages/hlsmanifest"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (manager *hlsManager) waitForHLSProjection(request *http.Request, root *os.Root, key, name string, duration float64) error {
	if err := request.Context().Err(); err != nil {
		return err
	}
	manager.mu.Lock()
	active := manager.jobs[key] != nil
	manager.mu.Unlock()
	if !active || invalidHLSProjectionDuration(duration) {
		return nil
	}
	if playID := jellyfinPlaySessionID(request); playID != "" && !validPlaybackSession(playID) {
		return nil
	}
	ctx, cancel := context.WithTimeout(request.Context(), 30*time.Second)
	defer cancel()
	if !playback.WaitHLSReady(ctx, func() error { return hlsProjectionReady(root, filepath.Join(key, filepath.FromSlash(name)), duration) }) {
		return errors.New("playlist timeline is not ready")
	}
	return nil
}

func hlsProjectionReady(root *os.Root, path string, duration float64) error {
	file, err := root.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	manifest, err := io.ReadAll(io.LimitReader(file, maxHLSPlaylistBytes+1))
	if err != nil {
		return err
	}
	if len(manifest) > maxHLSPlaylistBytes {
		return errors.New("playlist exceeds the size limit")
	}
	validated, err := hlsPlaylistWithSession(manifest, "")
	if err != nil {
		return err
	}
	projected, ready := hlsmanifest.CompleteVOD(validated, duration, 4)
	if _, err := hlsPlaylistWithSession(projected, ""); err != nil {
		return err
	}
	if !ready {
		return os.ErrNotExist
	}
	return nil
}

func invalidHLSProjectionDuration(duration float64) bool {
	return duration <= 0 || duration > 7*24*60*60 || math.IsNaN(duration) || math.IsInf(duration, 0)
}
