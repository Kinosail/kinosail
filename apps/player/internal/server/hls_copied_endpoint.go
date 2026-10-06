package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/playback"
)

type copiedHLSEndpointOutput struct{ copiedHLSProbeOutput }

func (output *copiedHLSEndpointOutput) Write(data []byte) (int, error) {
	if output.Len()+len(data) > 1<<20 {
		return 0, errCopiedHLSIndex
	}
	return output.Buffer.Write(data)
}

// Read the actual generated final fragment to EOF. Container format duration
// alone never authorizes extra media or a new URI.
func (manager *hlsManager) completedCopiedHLSEndpoint(ctx context.Context, directory string, manifest []byte) (float64, error) {
	last := ""
	for _, line := range strings.Split(string(manifest), "\n") {
		if _, valid := hlsSegmentNumber(line); valid {
			last = line
		}
	}
	if last == "" || !playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") {
		return 0, errCopiedHLSIndex
	}
	root, err := os.OpenRoot(directory) //nolint:gosec // Directory is a validated rendition cache path.
	if err != nil {
		return 0, errCopiedHLSIndex
	}
	defer root.Close()
	clock, err := manager.measureCopiedHLSClock(ctx, root)
	if err != nil {
		return 0, err
	}
	initialization, err := copiedHLSCacheFile(root, "init.mp4", 2<<20)
	if err != nil {
		return 0, err
	}
	fragment, err := copiedHLSCacheFile(root, last, 64<<20)
	if err != nil {
		return 0, err
	}
	output := copiedHLSEndpointOutput{}
	//nolint:gosec // Probe is installation config and input is bounded generated media.
	command := exec.CommandContext(ctx, manager.probe.executable, "-v", "error", "-threads", "1", "-select_streams", "v:0",
		"-show_packets", "-show_entries", "packet=pts_time,duration_time", "-of", "json", "pipe:0")
	command.Stdin = io.MultiReader(bytes.NewReader(initialization), bytes.NewReader(fragment))
	command.Stdout = &output
	if command.Run() != nil {
		return 0, errCopiedHLSIndex
	}
	var facts struct {
		Packets []struct {
			PTS      string `json:"pts_time"`
			Duration string `json:"duration_time"`
		} `json:"packets"`
	}
	if json.Unmarshal(output.Bytes(), &facts) != nil || len(facts.Packets) == 0 || len(facts.Packets) > 4096 {
		return 0, errCopiedHLSIndex
	}
	end := 0.0
	for _, packet := range facts.Packets {
		pts, ptsErr := strconv.ParseFloat(packet.PTS, 64)
		duration, durationErr := strconv.ParseFloat(packet.Duration, 64)
		if ptsErr != nil || durationErr != nil || math.IsNaN(pts) || math.IsNaN(duration) ||
			math.IsInf(pts, 0) || math.IsInf(duration, 0) || duration <= 0 {
			return 0, errCopiedHLSIndex
		}
		end = max(end, pts+duration)
	}
	return end - clock, nil
}

func (manager *hlsManager) completedCopiedHLSProjection(ctx context.Context, directory string, manifest []byte) []byte {
	end, err := manager.completedCopiedHLSEndpoint(ctx, directory, manifest)
	if err != nil {
		return manifest
	}
	return completedCopiedHLSManifest(manifest, end)
}

