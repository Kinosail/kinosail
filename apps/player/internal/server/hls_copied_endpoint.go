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
func (manager *hlsManager) completedCopiedHLSEndpoint(ctx context.Context, root *os.Root, manifest []byte) (float64, error) {
	last := copiedHLSLastSegment(manifest)
	if last == "" || !playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") || ctx.Err() != nil {
		return 0, errCopiedHLSIndex
	}
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
	end, err := decodeCopiedHLSEndpoint(output.Bytes())
	return end - clock, err
}

func (manager *hlsManager) completedCopiedHLSProjection(ctx context.Context, directory, policy string, manifest []byte) []byte {
	end, err := manager.copiedHLSEndpoint(ctx, directory, policy, manifest)
	if err != nil {
		return bytes.Replace(manifest, []byte("#EXT-X-PLAYLIST-TYPE:EVENT"), []byte("#EXT-X-PLAYLIST-TYPE:VOD"), 1)
	}
	return completedCopiedHLSManifest(manifest, end)
}

func copiedHLSLastSegment(manifest []byte) string {
	last := ""
	for _, line := range strings.Split(string(manifest), "\n") {
		if _, valid := hlsSegmentNumber(line); valid {
			last = line
		}
	}
	return last
}

func decodeCopiedHLSEndpoint(data []byte) (float64, error) {
	var facts struct {
		Packets []struct {
			PTS      string `json:"pts_time"`
			Duration string `json:"duration_time"`
		} `json:"packets"`
	}
	if json.Unmarshal(data, &facts) != nil || len(facts.Packets) == 0 || len(facts.Packets) > 4096 {
		return 0, errCopiedHLSIndex
	}
	end := 0.0
	for _, packet := range facts.Packets {
		packetEnd, err := copiedHLSPacketEnd(packet.PTS, packet.Duration)
		if err != nil {
			return 0, err
		}
		end = max(end, packetEnd)
	}
	return end, nil
}

func copiedHLSPacketEnd(ptsValue, durationValue string) (float64, error) {
	pts, ptsErr := strconv.ParseFloat(ptsValue, 64)
	duration, durationErr := strconv.ParseFloat(durationValue, 64)
	if ptsErr != nil || durationErr != nil || math.IsNaN(pts) || math.IsNaN(duration) ||
		math.IsInf(pts, 0) || math.IsInf(duration, 0) || duration <= 0 {
		return 0, errCopiedHLSIndex
	}
	return pts + duration, nil
}
