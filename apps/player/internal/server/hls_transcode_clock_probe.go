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
		if err := reader.open(interval); err != nil {
			return 0, errTranscodeHLSClock
		}
		n, err := reader.file.Read(data)
		reader.read += int64(n)
		if errors.Is(err, io.EOF) {
			if !reader.finish(interval) {
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

func (reader *transcodeFragmentReader) open(interval transcodeFragmentInterval) error {
	if reader.file != nil {
		return nil
	}
	file, info, err := copiedHLSOpenFile(reader.root, interval.name, 64<<20)
	if err != nil || !sameCopiedHLSFile(interval.info, info) {
		if file != nil {
			_ = file.Close()
		}
		return errTranscodeHLSClock
	}
	reader.file, reader.read = file, 0
	return nil
}

func (reader *transcodeFragmentReader) finish(interval transcodeFragmentInterval) bool {
	_ = reader.file.Close()
	reader.file = nil
	after, err := reader.root.Lstat(interval.name)
	return err == nil && sameCopiedHLSFile(interval.info, after) && reader.read == interval.end-interval.start
}

type transcodeClockProjection struct {
	intervals            []transcodeFragmentInterval
	clock                transcodeRenditionClock
	digests              [][2]hash.Hash
	streams              [2]bool
	terminal, shortAudio bool
}

func (projection *transcodeClockProjection) visit(line string) error {
	fields := mediaprobe.CompactFields(line)
	if strings.HasPrefix(line, "stream|") {
		return projection.stream(fields)
	}
	if !strings.HasPrefix(line, "packet|") {
		return transcodeClockError("probe-record", nil)
	}
	values, err := transcodePacketValues(fields)
	if err != nil {
		return err
	}
	stream, pts, dts, duration, position, size := values[0], values[1], values[2], values[3], values[4], values[5]
	owner := sort.Search(len(projection.intervals), func(index int) bool { return projection.intervals[index].end > position })
	if owner < 1 || owner >= len(projection.intervals) || position < projection.intervals[owner].start || size > projection.intervals[owner].end-position {
		return transcodePacketError("probe-owner", nil, values, owner, nil)
	}
	cut := &projection.clock.Cuts[owner-1]
	if !projection.audioDuration(stream, duration, owner) {
		return transcodePacketError("aac-packet-duration", nil, values, owner, cut)
	}
	if err := cut.addPacket(stream, pts, dts, fields["flags"]); err != nil {
		return transcodePacketError("probe-packets", err, values, owner, cut)
	}
	_, _ = fmt.Fprintf(projection.digests[owner-1][stream], "%d,%d,%d\n", pts, dts, duration)
	return nil
}

func transcodePacketValues(fields map[string]string) ([6]int64, error) {
	var values [6]int64
	for index, name := range []string{"stream_index", "pts", "dts", "duration", "pos", "size"} {
		value, err := strconv.ParseInt(fields[name], 10, 64)
		if err != nil || value < -(1<<40) || value > 1<<40 {
			return values, transcodeClockError("probe-field-"+name, nil)
		}
		values[index] = value
	}
	if values[0] < 0 || values[0] > 1 || values[3] <= 0 || values[5] <= 0 {
		return values, transcodePacketError("probe-values", nil, values, 0, nil)
	}
	return values, nil
}

func (cut *transcodeFragmentClock) addPacket(stream, pts, dts int64, flags string) error {
	if cut.Packets[stream] == 0 {
		if stream == 0 {
			if !strings.Contains(flags, "K") {
				return transcodeClockError("probe-keyframe", nil)
			}
			cut.VideoPTS, cut.VideoDTS = pts, dts
		} else {
			cut.AudioPTS = pts
		}
	}
	if stream == 1 && (pts != cut.AudioPTS+int64(cut.Packets[1])*1024 || dts != pts) {
		return transcodeClockError("probe-aac-sequence", nil)
	}
	cut.Packets[stream]++
	if cut.Packets[stream] > 16384 {
		return transcodeClockError("probe-packet-count", nil)
	}
	return nil
}

func (projection *transcodeClockProjection) audioDuration(stream, duration int64, owner int) bool {
	if stream != 1 {
		return true
	}
	if projection.shortAudio || duration <= 0 || duration > 1024 {
		return false
	}
	if duration == 1024 {
		return true
	}
	// The muxer can trim the final AAC packet. Only the actual final fragment
	// can contain it, and no later audio packet may follow in the joined demux.
	projection.shortAudio = true
	return projection.terminal && owner == len(projection.intervals)-1
}

func (projection *transcodeClockProjection) stream(fields map[string]string) error {
	index, err := strconv.Atoi(fields["index"])
	if err != nil || uint(index) > 1 || projection.streams[index] {
		return transcodeClockError("probe-stream", nil)
	}
	base := transcodeStreamBase(fields)
	if base == 0 {
		return transcodeClockError("probe-stream", nil)
	}
	switch {
	case index == 0 && fields["codec_type"] == "video":
		projection.clock.VideoBase = base
	case index == 1 && transcodeAACStream(fields, base):
		projection.clock.Rate = base
	default:
		return transcodeClockError("probe-stream", nil)
	}
	projection.streams[index] = true
	return nil
}

func transcodeStreamBase(fields map[string]string) int64 {
	base, err := strconv.ParseInt(strings.TrimPrefix(fields["time_base"], "1/"), 10, 32)
	if err != nil || base <= 0 || fields["time_base"] != "1/"+strconv.FormatInt(base, 10) {
		return 0
	}
	return base
}

func transcodeAACStream(fields map[string]string, base int64) bool {
	return fields["codec_type"] == "audio" && fields["codec_name"] == "aac" && fields["sample_rate"] == strconv.FormatInt(base, 10)
}

func (manager *hlsManager) probeTranscodeFragments(ctx context.Context, root *os.Root, names []string, terminal bool) (*transcodeRenditionClock, error) {
	if len(names) == 0 || len(names) > 16384 {
		return nil, errTranscodeHLSClock
	}
	projection := transcodeClockProjection{clock: transcodeRenditionClock{Cuts: make([]transcodeFragmentClock, len(names))}, terminal: terminal}
	intervals, err := transcodeFragmentIntervals(root, names)
	if err != nil {
		return nil, transcodeClockError("probe-interval", err)
	}
	projection.intervals = intervals
	for range names {
		projection.digests = append(projection.digests, [2]hash.Hash{sha256.New(), sha256.New()})
	}
	reader := &transcodeFragmentReader{ctx: ctx, root: root, intervals: projection.intervals}
	defer reader.Close()
	arguments := []string{"-v", "error", "-threads", "1", "-show_packets", "-show_streams", "-show_entries", "stream=index,codec_type,codec_name,time_base,sample_rate:stream_tags=:stream_disposition=:packet=stream_index,pts,dts,duration,pos,size,flags:packet_side_data=", "-of", "compact=p=1", "pipe:0"}
	logger := slog.With("request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx))
	if err := mediaprobe.ProbeInputLines(ctx, logger, manager.probe.executable, arguments, reader, 256<<20, 4_000_000, projection.visit); err != nil {
		return nil, transcodeClockError("probe-run", err)
	}
	if ctx.Err() != nil || reader.index != len(projection.intervals) {
		return nil, transcodeClockError("probe-consumption", nil)
	}
	if !currentTranscodeFragmentIntervals(root, projection.intervals) {
		return nil, transcodeClockError("probe-mutated", nil)
	}
	return projection.complete(root)
}

func (projection *transcodeClockProjection) complete(root *os.Root) (*transcodeRenditionClock, error) {
	if !projection.streams[0] || !projection.streams[1] {
		return nil, transcodeClockError("probe-streams", nil)
	}
	for index := range projection.clock.Cuts {
		cut := &projection.clock.Cuts[index]
		if cut.Packets[0] == 0 || cut.Packets[1] == 0 {
			return nil, transcodeClockError("probe-empty-track", nil)
		}
		for stream := range 2 {
			copy(cut.Clocks[stream][:], projection.digests[index][stream].Sum(nil))
		}
	}
	initialization, err := copiedHLSCacheFile(root, "init.mp4", 2<<20)
	if err != nil {
		return nil, transcodeClockError("probe-init", err)
	}
	projection.clock.Initialization = sha256.Sum256(initialization)
	return &projection.clock, nil
}

func transcodeFragmentIntervals(root *os.Root, names []string) ([]transcodeFragmentInterval, error) {
	var intervals []transcodeFragmentInterval
	var position int64
	for _, name := range append([]string{"init.mp4"}, names...) {
		info, err := root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 64<<20 || position > (64<<30)-info.Size() {
			return nil, errTranscodeHLSClock
		}
		intervals = append(intervals, transcodeFragmentInterval{name, position, position + info.Size(), info})
		position += info.Size()
	}
	return intervals, nil
}

func currentTranscodeFragmentIntervals(root *os.Root, intervals []transcodeFragmentInterval) bool {
	for _, interval := range intervals {
		current, err := root.Lstat(interval.name)
		if err != nil || !sameCopiedHLSFile(interval.info, current) {
			return false
		}
	}
	return true
}
