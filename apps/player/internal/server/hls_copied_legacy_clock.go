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
)

func (manager *hlsManager) measureCopiedHLSClock(ctx context.Context, root *os.Root) (float64, error) {
	if ctx.Err() != nil {
		return 0, errCopiedHLSIndex
	}
	initialization, err := copiedHLSCacheFile(root, "init.mp4", 2<<20)
	if err != nil {
		return 0, err
	}
	fragment, err := copiedHLSCacheFile(root, "segment-00000.m4s", 64<<20)
	if err != nil {
		return 0, err
	}
	output := copiedHLSProbeOutput{}
	//nolint:gosec // Probe is installation config; input is bounded, rooted generated media.
	command := exec.CommandContext(ctx, manager.probe.executable, "-v", "error", "-threads", "1", "-select_streams", "v:0",
		"-read_intervals", "%+#8", "-show_packets", "-show_entries", "packet=pts_time,flags", "-of", "json", "pipe:0")
	command.Stdin = io.MultiReader(bytes.NewReader(initialization), bytes.NewReader(fragment))
	command.Stdout = &output
	if command.Run() != nil {
		return 0, errCopiedHLSIndex
	}
	return decodeCopiedHLSClock(output.Bytes())
}

func decodeCopiedHLSClock(data []byte) (float64, error) {
	var facts struct {
		Packets []struct {
			PTS   string `json:"pts_time"`
			Flags string `json:"flags"`
		} `json:"packets"`
	}
	if json.Unmarshal(data, &facts) != nil || len(facts.Packets) == 0 ||
		!strings.Contains(facts.Packets[0].Flags, "K") {
		return 0, errCopiedHLSIndex
	}
	clock, err := strconv.ParseFloat(facts.Packets[0].PTS, 64)
	if err != nil || math.IsNaN(clock) || clock < 0 || clock > 1 {
		return 0, errCopiedHLSIndex
	}
	return clock, nil
}
