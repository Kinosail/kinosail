package server

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/mediaprobe"
)

var errTranscodeHLSClock = errors.New("transcoded HLS recovery clock unavailable")

type transcodeFragmentClock struct {
	VideoPTS, VideoDTS, AudioPTS int64
	Packets                      [2]int
	Clocks                       [2][32]byte
}

type transcodeRenditionClock struct {
	Initialization, Manifest [32]byte
	VideoBase, Rate          int64
	Cuts                     []transcodeFragmentClock
}

type transcodeFragmentInterval struct {
	name       string
	start, end int64
	info       os.FileInfo
}

// Open only the current immutable fragment. Retain neither a whole movie nor a
// descriptor per fragment while feeding one joined demux pass.
type transcodeFragmentReader struct {
	ctx       context.Context
	root      *os.Root
	intervals []transcodeFragmentInterval
	index     int
	file      *os.File
	read      int64
}

func (reader *transcodeFragmentReader) Close() error {
	if reader.file != nil {
		return reader.file.Close()
	}
	return nil
}

func (reader *transcodeFragmentReader) Read(data []byte) (int, error) {
	for reader.index < len(reader.intervals) {
		if err := reader.ctx.Err(); err != nil {
			return 0, err
		}
		interval := reader.intervals[reader.index]
		if reader.file == nil {
			file, info, err := copiedHLSOpenFile(reader.root, interval.name, 64<<20)
			if err != nil || !sameCopiedHLSFile(interval.info, info) {
				if file != nil {
					_ = file.Close()
				}
				return 0, errTranscodeHLSClock
			}
			reader.file, reader.read = file, 0
		}
		n, err := reader.file.Read(data)
		reader.read += int64(n)
		if errors.Is(err, io.EOF) {
			_ = reader.file.Close()
			reader.file = nil
			after, statErr := reader.root.Lstat(interval.name)
			if statErr != nil || !sameCopiedHLSFile(interval.info, after) || reader.read != interval.end-interval.start {
				return n, errTranscodeHLSClock
			}
			reader.index++
			if n == 0 {
				continue
			}
			err = nil
		}
		return n, err
	}
	return 0, io.EOF
}

type transcodeClockProjection struct {
	intervals []transcodeFragmentInterval
	clock     transcodeRenditionClock
	digests   [][2]hash.Hash
	streams   [2]bool
}

func (projection *transcodeClockProjection) visit(line string) error {
	fields := mediaprobe.CompactFields(line)
	if strings.HasPrefix(line, "stream|") {
		index, err := strconv.Atoi(fields["index"])
		if err != nil || index < 0 || index > 1 || projection.streams[index] {
			return errTranscodeHLSClock
		}
		base, err := strconv.ParseInt(strings.TrimPrefix(fields["time_base"], "1/"), 10, 32)
		if err != nil || base <= 0 || fields["time_base"] != "1/"+strconv.FormatInt(base, 10) {
			return errTranscodeHLSClock
		}
		if index == 0 && fields["codec_type"] == "video" {
			projection.clock.VideoBase = base
		} else if index == 1 && fields["codec_type"] == "audio" && fields["codec_name"] == "aac" && fields["sample_rate"] == strconv.FormatInt(base, 10) {
			projection.clock.Rate = base
		} else {
			return errTranscodeHLSClock
		}
		projection.streams[index] = true
		return nil
	}
	if !strings.HasPrefix(line, "packet|") {
		return errTranscodeHLSClock
	}
	var values [6]int64
	for index, name := range []string{"stream_index", "pts", "dts", "duration", "pos", "size"} {
		value, err := strconv.ParseInt(fields[name], 10, 64)
		if err != nil || value < -(1<<40) || value > 1<<40 {
			return errTranscodeHLSClock
		}
		values[index] = value
	}
	stream, pts, dts, duration, position, size := values[0], values[1], values[2], values[3], values[4], values[5]
	owner := sort.Search(len(projection.intervals), func(index int) bool { return projection.intervals[index].end > position })
	if stream < 0 || stream > 1 || duration <= 0 || size <= 0 || owner < 1 || owner >= len(projection.intervals) || position < projection.intervals[owner].start || size > projection.intervals[owner].end-position {
		return errTranscodeHLSClock
	}
	cut := &projection.clock.Cuts[owner-1]
	if cut.Packets[stream] == 0 {
		if stream == 0 {
			if !strings.Contains(fields["flags"], "K") {
				return errTranscodeHLSClock
			}
			cut.VideoPTS, cut.VideoDTS = pts, dts
		} else {
			cut.AudioPTS = pts
		}
	}
	if stream == 1 && (duration != 1024 || pts != cut.AudioPTS+int64(cut.Packets[1])*1024 || dts != pts) {
		return errTranscodeHLSClock
	}
	cut.Packets[stream]++
	if cut.Packets[stream] > 16384 {
		return errTranscodeHLSClock
	}
	_, _ = fmt.Fprintf(projection.digests[owner-1][stream], "%d,%d,%d\n", pts, dts, duration)
	return nil
}

func (manager *hlsManager) probeTranscodeFragments(ctx context.Context, root *os.Root, names []string) (*transcodeRenditionClock, error) {
	if len(names) == 0 || len(names) > 16384 {
		return nil, errTranscodeHLSClock
	}
	projection := transcodeClockProjection{clock: transcodeRenditionClock{Cuts: make([]transcodeFragmentClock, len(names))}}
	var position int64
	for _, name := range append([]string{"init.mp4"}, names...) {
		info, err := root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 64<<20 || position > (64<<30)-info.Size() {
			return nil, errTranscodeHLSClock
		}
		projection.intervals = append(projection.intervals, transcodeFragmentInterval{name, position, position + info.Size(), info})
		position += info.Size()
	}
	for range names {
		projection.digests = append(projection.digests, [2]hash.Hash{sha256.New(), sha256.New()})
	}
	reader := &transcodeFragmentReader{ctx: ctx, root: root, intervals: projection.intervals}
	defer reader.Close()
	arguments := []string{"-v", "error", "-threads", "1", "-show_packets", "-show_streams", "-show_entries", "stream=index,codec_type,codec_name,time_base,sample_rate:stream_tags=:stream_disposition=:packet=stream_index,pts,dts,duration,pos,size,flags:packet_side_data=", "-of", "compact=p=1", "pipe:0"}
	logger := slog.With("request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx))
	if err := mediaprobe.ProbeInputLines(ctx, logger, manager.probe.executable, arguments, reader, 256<<20, 4_000_000, projection.visit); err != nil {
		return nil, err
	}
	if ctx.Err() != nil || reader.index != len(projection.intervals) || !projection.streams[0] || !projection.streams[1] {
		return nil, errTranscodeHLSClock
	}
	for _, interval := range projection.intervals {
		current, err := root.Lstat(interval.name)
		if err != nil || !sameCopiedHLSFile(interval.info, current) {
			return nil, errTranscodeHLSClock
		}
	}
	for index := range projection.clock.Cuts {
		cut := &projection.clock.Cuts[index]
		if cut.Packets[0] == 0 || cut.Packets[1] == 0 {
			return nil, errTranscodeHLSClock
		}
		for stream := range 2 {
			copy(cut.Clocks[stream][:], projection.digests[index][stream].Sum(nil))
		}
	}
	initialization, err := copiedHLSCacheFile(root, "init.mp4", 2<<20)
	if err != nil {
		return nil, errTranscodeHLSClock
	}
	projection.clock.Initialization = sha256.Sum256(initialization)
	return &projection.clock, nil
}
