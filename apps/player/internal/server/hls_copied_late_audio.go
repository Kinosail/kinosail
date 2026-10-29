package server

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

const lateCopiedAudioSeconds = 1.0

func (manager *hlsManager) remuxAudioArguments(ctx context.Context, item library.Item, recipe hlsRecipe) []string {
	if recipe.mode != "remux" || item.Kind == "audio" || item.Kind == "audiobook" {
		return nil
	}
	for _, track := range manager.probe.facts(ctx, item).AudioFacts {
		if track.Index != recipe.audio || track.Codec != "ac3" && track.Codec != "eac3" || !manager.copiedAudioStartsLate(ctx, item, recipe.audio) {
			continue
		}
		arguments := []string{"-af", "aresample=async=1:first_pts=0", "-c:a", track.Codec, "-b:a", "640k"}
		if track.Channels > 6 {
			arguments = append(arguments, "-ac", "6")
		}
		slog.InfoContext(ctx, "HLS copied audio", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "outcome", "encoded-late-start", "codec", track.Codec)
		return append(arguments, "-max_interleave_delta", "0", "-max_muxing_queue_size", "8192")
	}
	return nil
}

func (manager *hlsManager) copiedAudioStartsLate(ctx context.Context, item library.Item, audio int) bool {
	key := item.ID + "/" + strconv.FormatInt(item.Size, 10) + "/" + strconv.Itoa(audio)
	metadata := &manager.copiedMetadata
	metadata.mu.Lock()
	late, known := metadata.lateAudio[key]
	metadata.mu.Unlock()
	if known {
		return late
	}
	probe, cancel := copiedHLSAudioProofContext(ctx)
	defer cancel()
	video, videoFound := manager.firstPacketTime(probe, item.Path, "v:0")
	sound, soundFound := manager.firstPacketTime(probe, item.Path, "a:"+strconv.Itoa(audio))
	if ctx.Err() != nil || !videoFound {
		return false
	}
	late = !soundFound && probe.Err() != nil || soundFound && sound-video > lateCopiedAudioSeconds
	metadata.mu.Lock()
	if metadata.lateAudio == nil {
		metadata.lateAudio = make(map[string]bool)
	}
	metadata.lateAudio[key] = late
	metadata.mu.Unlock()
	return late
}

func (manager *hlsManager) firstPacketTime(ctx context.Context, path, stream string) (float64, bool) {
	value := ""
	arguments := []string{"-v", "error", "-threads", "1", "-select_streams", stream, "-read_intervals", "%+#1", "-show_entries", "packet=pts_time", "-of", "csv=p=0", path}
	err := copiedHLSLines(ctx, manager.probe.executable, arguments, 4<<10, 4, func(line string) error {
		if value == "" {
			value = strings.Trim(strings.TrimSpace(line), ",")
		}
		return nil
	})
	seconds, parseErr := strconv.ParseFloat(value, 64)
	return seconds, err == nil && parseErr == nil
}
