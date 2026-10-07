package server

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/workload"
)

// AAC encoding adds one delay frame and rounds the tail to a complete frame.
// Only actual decoded AAC samples can admit the completed final ordinal beyond
// the source window; neither a source sample rate nor a manifest alone can.
func (manager *hlsManager) completedAudioHLSFinalSegment(ctx context.Context, item library.Item, directory, name string, manifest []byte, offset, playable float64) bool {
	if !slices.Contains([]string{"audio", "audiobook"}, item.Kind) || filepath.Base(name) != copiedHLSLastSegment(manifest) ||
		!playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") || !manager.initializationsReady(filepath.Dir(directory)) {
		return false
	}
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return false
	}
	defer root.Close()
	assets, err := copiedHLSEndpointAssets(root, manifest)
	if err != nil {
		return false
	}
	data, err := manager.completedAudioHLSFrames(ctx, root, filepath.Base(name))
	if err != nil || !validCompletedAACFrames(data, offset, playable) {
		slog.WarnContext(ctx, "HLS audio terminal media rejected", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "failure_class", "invalid-terminal-metadata")
		return false
	}
	current, err := copiedHLSEndpointAssets(root, manifest)
	return err == nil && sameCopiedHLSAssets(assets, current)
}

func (manager *hlsManager) completedAudioHLSFrames(ctx context.Context, root *os.Root, name string) ([]byte, error) {
	initialization, err := copiedHLSCacheFile(root, "init.mp4", 2<<20)
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	fragment, err := copiedHLSCacheFile(root, filepath.Base(name), 64<<20)
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	ctx, release, err := manager.copiedHLSMetadataAdmission(ctx)
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	defer release()
	releaseWork, err := manager.workloads.Acquire(ctx, workload.Playback)
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	defer releaseWork()
	output := copiedHLSEndpointOutput{}
	//nolint:gosec // Installation-configured probe and bounded rooted generated media.
	command := exec.CommandContext(ctx, manager.probe.executable, "-v", "error", "-threads", "1", "-select_streams", "a:0",
		"-show_frames", "-show_entries", "stream=codec_name,profile,sample_rate:frame=pts_time,nb_samples", "-of", "json", "pipe:0")
	command.Stdin = io.MultiReader(bytes.NewReader(initialization), bytes.NewReader(fragment))
	command.Stdout = &output
	if command.Run() != nil {
		return nil, errCopiedHLSIndex
	}
	return output.Bytes(), nil
}

func validCompletedAACFrames(data []byte, offset, playable float64) bool {
	var facts completedAACFacts
	if httpguard.DecodeUniqueJSON(bytes.NewReader(data), 1<<20, &facts) != nil || !validAACFrameCollections(facts) {
		return false
	}
	rate, valid := completedAACSampleRate(facts)
	if !valid {
		return false
	}
	tick, frame := 1/float64(rate), 1024/float64(rate)
	end := offset
	for _, decoded := range facts.Frames {
		pts, next, valid := completedAACFrameEnd(decoded.PTS, decoded.Samples, rate)
		if !valid || math.Abs(pts-end) > tick {
			return false
		}
		end = next
	}
	return end > offset && end <= playable+2*frame
}

func completedAACFrameEnd(value string, samples, rate int) (float64, float64, bool) {
	pts, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(pts) || math.IsInf(pts, 0) || samples < 1 || samples > 1024 {
		return 0, 0, false
	}
	return pts, pts + float64(samples)/float64(rate), true
}

func validAACSampleRate(rate int) bool {
	switch rate {
	case 7350, 8000, 11025, 12000, 16000, 22050, 24000, 32000, 44100, 48000, 64000, 88200, 96000:
		return true
	}
	return false
}

type completedAACFacts struct {
	Streams []struct {
		Codec   string `json:"codec_name"`
		Rate    string `json:"sample_rate"`
		Profile string `json:"profile"`
	} `json:"streams"`
	Programs     []struct{} `json:"programs"`
	StreamGroups []struct{} `json:"stream_groups"`
	Frames       []struct {
		PTS     string `json:"pts_time"`
		Samples int    `json:"nb_samples"`
	} `json:"frames"`
}

func validAACFrameCollections(facts completedAACFacts) bool {
	return len(facts.Streams) == 1 && len(facts.Programs) == 0 && len(facts.StreamGroups) == 0 && len(facts.Frames) > 0 && len(facts.Frames) <= 4096
}

func completedAACSampleRate(facts completedAACFacts) (int, bool) {
	stream := facts.Streams[0]
	if stream.Codec != "aac" || stream.Profile != "LC" {
		return 0, false
	}
	rate, err := strconv.Atoi(stream.Rate)
	return rate, err == nil && strconv.Itoa(rate) == stream.Rate && validAACSampleRate(rate)
}
