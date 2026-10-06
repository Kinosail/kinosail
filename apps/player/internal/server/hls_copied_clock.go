package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

type copiedHLSProbeOutput struct {
	bytes.Buffer
}

func (output *copiedHLSProbeOutput) Write(data []byte) (int, error) {
	if output.Len()+len(data) > 8<<10 {
		return 0, errCopiedHLSIndex
	}
	return output.Buffer.Write(data)
}

func (manager *hlsManager) ensureCopiedHLSClock(ctx context.Context, item library.Item, recipe hlsRecipe, directory, policy string) error {
	manager.copyTimelineMu.Lock()
	defer manager.copyTimelineMu.Unlock()
	timeline, err := readCopiedHLSTimeline(directory, policy)
	if err != nil {
		return nil // Ordinary cold streams have no indexed strategy.
	}
	if timeline.Clock != nil {
		return nil
	}
	master, err := playback.ReadHLSPlaylist(filepath.Join(directory, "index.m3u8"))
	if err != nil {
		return errCopiedHLSIndex
	}
	for _, rendition := range strings.Split(string(master), "\n") {
		if !hlsFile(rendition) || !strings.HasSuffix(rendition, "/index.m3u8") {
			continue
		}
		root, err := os.OpenRoot(filepath.Join(directory, filepath.Dir(rendition))) //nolint:gosec // Master URI passed the rendition allowlist.
		if err != nil {
			return errCopiedHLSIndex
		}
		clock, probeErr := manager.measureCopiedHLSClock(ctx, root)
		_ = root.Close()
		if probeErr != nil || manager.validateHLSPolicy(ctx, item, recipe, policy) != nil {
			return errCopiedHLSIndex
		}
		timeline.Clock = &clock
		return writeCopiedHLSTimeline(directory, timeline)
	}
	return errCopiedHLSIndex
}

func (manager *hlsManager) measureCopiedHLSClock(ctx context.Context, root *os.Root) (float64, error) {
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
	var facts struct {
		Packets []struct {
			PTS   string `json:"pts_time"`
			Flags string `json:"flags"`
		} `json:"packets"`
	}
	if json.Unmarshal(output.Bytes(), &facts) != nil || len(facts.Packets) == 0 ||
		!strings.Contains(facts.Packets[0].Flags, "K") {
		return 0, errCopiedHLSIndex
	}
	clock, err := strconv.ParseFloat(facts.Packets[0].PTS, 64)
	if err != nil || math.IsNaN(clock) || clock < 0 || clock > 1 {
		return 0, errCopiedHLSIndex
	}
	return clock, nil
}

func indexedCopiedHLSSegmentArguments(arguments []string, timeline *copiedHLSTimeline) []string {
	if timeline != nil {
		for number := range arguments {
			if arguments[number] == "-hls_time" && number+1 < len(arguments) {
				arguments[number+1] = "0.1"
			}
		}
	}
	return arguments
}
