package server

import (
	"bytes"
	"context"
	"io"
	"os"
	"strconv"

	"github.com/MikeO7/kinosail/packages/library"
)

// The caller owns one inherited two-second lease and rooted generation guards.
func (manager *hlsManager) measureCopiedAACOrigin(ctx context.Context, item library.Item, recipe hlsRecipe, policy string, timeline *copiedHLSTimeline, media *os.Root) (*copiedHLSAudioOrigin, error) {
	pending := timeline.AudioOrigin
	if pending == nil || pending.FirstHash != "" || timeline.Clock != nil {
		return nil, errCopiedHLSIndex
	}
	before, err := manager.copiedHLSSourceAudioIdentity(ctx, item, recipe, policy, nil)
	if err != nil {
		return nil, err
	}
	file, source, err := openCopiedHLSSourceAudio(ctx, item.Path, before)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	selected, track, err := copiedAACSourceGrid(ctx, manager.probe.executable, source)
	if err != nil || !selected || track != pending.SourceTrack {
		return nil, errCopiedHLSIndex
	}
	start := max(int64(0), pending.InitialSeekMicros-1_000_000)
	rows, err := copiedAACPackets(ctx, manager.probe.executable, source, copiedAACMicrosText(start)+"%+#128", nil, false, pending.SourceTrack)
	if err != nil {
		return nil, err
	}
	initialization, err := copiedHLSCacheFile(media, "init.mp4", 2<<20)
	if err != nil {
		return nil, err
	}
	first, err := copiedHLSCacheFile(media, "segment-00000.m4s", 64<<20)
	if err != nil {
		return nil, err
	}
	input := func() io.Reader { return io.MultiReader(bytes.NewReader(initialization), bytes.NewReader(first)) }
	edited, editErr := copiedAACPackets(ctx, manager.probe.executable, "pipe:0", "%+#8", input(), false, -1)
	raw, rawErr := copiedAACPackets(ctx, manager.probe.executable, "pipe:0", "%+#8", input(), true, -1)
	if editErr != nil || rawErr != nil || len(edited) == 0 || len(raw) == 0 {
		return nil, errCopiedHLSIndex
	}
	origin, err := copiedHLSAudioWitness(rows, edited[0], raw[0], pending.InitialSeekMicros)
	if err != nil || !manager.copiedHLSSourceAudioComplete(ctx, item, recipe, policy, before, file) {
		return nil, errCopiedHLSIndex
	}
	origin.SourceTrack = pending.SourceTrack
	return origin, nil
}

func copiedAACPackets(ctx context.Context, executable, source, interval string, input io.Reader, raw bool, expectedTrack int) ([]copiedHLSAudioPacket, error) {
	rows := make([]copiedHLSAudioPacket, 0, 128)
	streams := 0
	arguments := []string{"-v", "error", "-threads", "1"}
	if raw {
		arguments = append(arguments, "-ignore_editlist", "1")
	}
	arguments = append(arguments, "-select_streams", "a:0", "-read_intervals", interval, "-show_packets", "-show_streams",
		"-show_data_hash", "sha256", "-show_entries", "packet=pts,dts,duration,data_hash:stream=index,codec_name,profile,sample_rate,channels,time_base", "-of", "compact=p=0", source)
	err := copiedHLSInputLines(ctx, executable, arguments, input, 64<<10, 144, func(line string) error {
		fields := copiedHLSFields(line)
		if codec, ok := fields["codec_name"]; ok {
			index, err := strconv.Atoi(fields["index"])
			if err != nil || streams != 0 || codec != "aac" || fields["profile"] != "LC" || fields["sample_rate"] != "48000" ||
				fields["channels"] != "2" || fields["time_base"] != "1/48000" || expectedTrack >= 0 && index != expectedTrack {
				return errCopiedHLSIndex
			}
			streams++
			return nil
		}
		pts, ptsErr := strconv.ParseInt(fields["pts"], 10, 64)
		dts, dtsErr := strconv.ParseInt(fields["dts"], 10, 64)
		duration, durationErr := strconv.ParseInt(fields["duration"], 10, 64)
		packet := copiedHLSAudioPacket{PTS: pts, DTS: dts, Duration: duration, Hash: fields["data_hash"]}
		if ptsErr != nil || dtsErr != nil || durationErr != nil || !validCopiedAACPacket(packet) || len(rows) >= 128 {
			return errCopiedHLSIndex
		}
		rows = append(rows, packet)
		return nil
	})
	if err != nil || streams != 1 || len(rows) == 0 || ctx.Err() != nil {
		return nil, errCopiedHLSIndex
	}
	return rows, nil
}

func (manager *hlsManager) measureCopiedAACVideoClock(ctx context.Context, media *os.Root) (float64, error) {
	initialization, err := copiedHLSCacheFile(media, "init.mp4", 2<<20)
	if err != nil {
		return 0, err
	}
	first, err := copiedHLSCacheFile(media, "segment-00000.m4s", 64<<20)
	if err != nil {
		return 0, err
	}
	rows, valid := 0, false
	arguments := []string{"-v", "error", "-threads", "1", "-select_streams", "v:0", "-read_intervals", "%+#8", "-show_packets",
		"-show_entries", "packet=pts,flags", "-of", "compact=p=0", "pipe:0"}
	err = copiedHLSInputLines(ctx, manager.probe.executable, arguments, io.MultiReader(bytes.NewReader(initialization), bytes.NewReader(first)), 8<<10, 16, func(line string) error {
		fields := copiedHLSFields(line)
		if rows == 0 {
			valid = fields["pts"] == "0" && bytes.Contains([]byte(fields["flags"]), []byte("K"))
		}
		rows++
		return nil
	})
	if err != nil || !valid || rows == 0 || ctx.Err() != nil {
		return 0, errCopiedHLSIndex
	}
	return 0, nil
}
