package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (manager *hlsManager) openHLSDeliveryFile(request *http.Request, root *os.Root, item library.Item, recipe hlsRecipe, key, name string, plan *copiedHLSPlan) (*os.File, error) {
	file, err := root.Open(filepath.FromSlash(name))
	if errors.Is(err, os.ErrNotExist) && manager.waitForPublishedHLSFile(request, root, item, recipe, key, name, plan) {
		return root.Open(filepath.FromSlash(name))
	}
	return file, err
}

func (manager *hlsManager) waitForPublishedHLSFile(request *http.Request, root *os.Root, item library.Item, recipe hlsRecipe, key, name string, plan *copiedHLSPlan) bool {
	if !hlsFile(name) || !strings.HasSuffix(name, ".m4s") {
		return false
	}
	manager.mu.Lock()
	active := manager.jobs[key] != nil
	manager.mu.Unlock()
	if !active || !manager.hlsSegmentPublished(request, root, item, recipe, name, plan) {
		return false
	}
	ctx, cancel := context.WithTimeout(request.Context(), 30*time.Second)
	defer cancel()
	return playback.WaitHLSReady(ctx, func() error {
		_, err := root.Stat(filepath.FromSlash(name))
		return err
	})
}

func (manager *hlsManager) hlsSegmentPublished(request *http.Request, root *os.Root, item library.Item, recipe hlsRecipe, name string, plan *copiedHLSPlan) bool {
	file, err := root.Open(filepath.Join(filepath.Dir(filepath.FromSlash(name)), "index.m3u8"))
	if err != nil {
		return false
	}
	defer file.Close()
	manifest, err := io.ReadAll(io.LimitReader(file, maxHLSPlaylistBytes+1))
	if err != nil || len(manifest) > maxHLSPlaylistBytes {
		return false
	}
	validated, err := hlsPlaylistWithSession(manifest, "")
	if err != nil {
		return false
	}
	if playID := jellyfinPlaySessionID(request); playID != "" && !validPlaybackSession(playID) {
		return strings.Contains("\n"+string(validated), "\n"+filepath.Base(name)+"\n")
	}
	duration := playback.HLSPlaybackDuration(sharedHLSRecipe(recipe), manager.probe.duration(request.Context(), item))
	completed, _, err := plan.project(validated, duration)
	if err != nil {
		return false
	}
	projected, err := hlsPlaylistWithSession(completed, "")
	if err != nil {
		return false
	}
	return strings.Contains("\n"+string(projected), "\n"+filepath.Base(name)+"\n")
}
