package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/playback"
)

const maximumCopiedHLSTimelineBytes = 256 << 10

func validCopiedHLSTimeline(timeline *copiedHLSTimeline) bool {
	if !validCopiedHLSHeader(timeline) || !validCopiedHLSTimeBase(timeline) || !validCopiedHLSEnd(timeline.End) {
		return false
	}
	for number := range timeline.Keys {
		if !validCopiedHLSCut(timeline, number) {
			return false
		}
	}
	return timeline.Clock == nil || !math.IsNaN(*timeline.Clock) && *timeline.Clock >= 0 && *timeline.Clock <= 1
}

func validCopiedHLSHeader(timeline *copiedHLSTimeline) bool {
	return timeline != nil && timeline.Strategy == "h264-idr-keys-1" && timeline.Policy != "" && len(timeline.Policy) <= 16<<10 &&
		len(timeline.Keys) > 0 && len(timeline.Keys) <= maximumCopiedHLSKeys
}

func validCopiedHLSTimeBase(timeline *copiedHLSTimeline) bool {
	return timeline.Numerator > 0 && timeline.Denominator > 0 &&
		timeline.TimeBase == float64(timeline.Numerator)/float64(timeline.Denominator) &&
		timeline.TimeBase > 0 && timeline.TimeBase <= 0.001 && !math.IsNaN(timeline.TimeBase)
}

func validCopiedHLSEnd(end float64) bool {
	return end > 0 && end <= 7*24*60*60 && !math.IsNaN(end) && !math.IsInf(end, 0)
}

func validCopiedHLSCut(timeline *copiedHLSTimeline, number int) bool {
	point := timeline.point(number)
	if point < 0 || point >= timeline.End || number > 0 && point-timeline.point(number-1) < 0.25 {
		return false
	}
	return !invalidHLSSegmentDuration(timeline.segmentEnd(number) - point)
}

func (timeline *copiedHLSTimeline) segmentEnd(number int) float64 {
	if number+1 < len(timeline.Keys) {
		return timeline.point(number + 1)
	}
	return timeline.End
}

func copiedHLSCacheFile(root *os.Root, name string, limit int64) ([]byte, error) {
	file, info, err := copiedHLSOpenFile(root, name, limit)
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	after, statErr := root.Lstat(name)
	if err != nil || statErr != nil || int64(len(data)) != info.Size() || !sameCopiedHLSFile(info, after) {
		return nil, errCopiedHLSIndex
	}
	return data, nil
}

func (manager *hlsManager) readCopiedHLSTimeline(directory, policy string) (*copiedHLSTimeline, error) {
	return manager.readCopiedHLSTimelineContext(manager.ctx, directory, policy)
}

func (manager *hlsManager) readCopiedHLSTimelineContext(ctx context.Context, directory, policy string) (*copiedHLSTimeline, error) {
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	defer root.Close()
	binding, err := copiedHLSCacheFile(root, ".source", 16<<10)
	if err != nil || string(binding) != policy {
		return nil, errCopiedHLSIndex
	}
	data, err := copiedHLSCacheFile(root, ".copy-timeline", maximumCopiedHLSTimelineBytes)
	var timeline copiedHLSTimeline
	if err != nil || httpguard.DecodeUniqueJSON(bytes.NewReader(data), maximumCopiedHLSTimelineBytes, &timeline) != nil || timeline.Policy != policy || !validCopiedHLSTimeline(&timeline) {
		return nil, errCopiedHLSIndex
	}
	if ctx.Err() != nil {
		return nil, errCopiedHLSIndex
	}
	if timeline.Clock != nil && manager.verifyCopiedHLSCertificate(ctx, directory, root, data, &timeline) != nil {
		return nil, errCopiedHLSIndex
	}
	return &timeline, nil
}

func (manager *hlsManager) writeCopiedHLSTimeline(directory string, timeline *copiedHLSTimeline) error {
	if !validCopiedHLSTimeline(timeline) {
		return errCopiedHLSIndex
	}
	data, err := json.Marshal(timeline)
	if err != nil || len(data) > maximumCopiedHLSTimelineBytes {
		return errCopiedHLSIndex
	}
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer root.Close()
	binding, err := copiedHLSCacheFile(root, ".source", 16<<10)
	if err != nil || string(binding) != timeline.Policy {
		return errCopiedHLSIndex
	}
	return writeCopiedHLSMetadata(root, ".copy-timeline", data)
}

func writeCopiedHLSMetadata(root *os.Root, name string, data []byte) error {
	if name != ".copy-timeline" && name != ".copy-clock" {
		return errCopiedHLSIndex
	}
	file, err := root.OpenFile(name+".pending", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer func() {
		_ = file.Close()
		_ = root.Remove(name + ".pending")
	}()
	_, err = file.Write(data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return errCopiedHLSIndex
	}
	return root.Rename(name+".pending", name)
}

func copiedHLSManifest(manifest []byte, timeline *copiedHLSTimeline) ([]byte, bool) {
	if !validCopiedHLSTimeline(timeline) || timeline.Clock == nil ||
		!playback.PlaylistHas(manifest, "#EXT-X-PLAYLIST-TYPE:EVENT") ||
		!bytes.Contains(manifest, []byte("#EXT-X-MAP:URI=\"init.mp4\"")) {
		return manifest, false
	}
	if !matchesCopiedHLSManifest(manifest, timeline) {
		return manifest, false
	}
	target := 1.0
	for number := range timeline.Keys {
		end := timeline.segmentEnd(number)
		target = max(target, math.Ceil(end-timeline.point(number)))
	}
	var output strings.Builder
	output.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:" + strconv.FormatFloat(target, 'f', 0, 64) +
		"\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-MAP:URI=\"init.mp4\"\n")
	for number := range timeline.Keys {
		end := timeline.segmentEnd(number)
		output.WriteString("#EXTINF:" + strconv.FormatFloat(end-timeline.point(number), 'f', 6, 64) +
			",\nsegment-" + copiedHLSSegmentDigits(number) + ".m4s\n")
	}
	output.WriteString("#EXT-X-ENDLIST\n")
	return []byte(output.String()), true
}

func copiedHLSSegmentDigits(number int) string {
	value := strconv.Itoa(number)
	return strings.Repeat("0", 5-len(value)) + value
}

func copiedHLSTime(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func matchesCopiedHLSManifest(manifest []byte, timeline *copiedHLSTimeline) bool {
	length, observed := 0.0, 0
	for _, line := range strings.Split(string(manifest), "\n") {
		if strings.HasPrefix(line, "#EXTINF:") {
			value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
			var err error
			length, err = strconv.ParseFloat(value, 64)
			if err != nil {
				return false
			}
		}
		number, ok := hlsSegmentNumber(line)
		if !ok {
			continue
		}
		if !matchesCopiedHLSSegment(timeline, number, observed, length) {
			return false
		}
		observed++
		length = 0
	}
	if observed == 0 || playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") && observed != len(timeline.Keys) {
		return false
	}
	return true
}

func matchesCopiedHLSLength(timeline *copiedHLSTimeline, number int, length float64) bool {
	expected := timeline.segmentEnd(number) - timeline.point(number)
	tolerance := 0.002
	if number+1 == len(timeline.Keys) {
		tolerance = min(1, max(0.1, expected*0.05)) // Demux duration ticks can round down across the last GOP.
	}
	return math.Abs(length-expected) <= tolerance
}

func validCopiedHLSManifestCut(lines []string, number int, length float64) bool {
	if invalidHLSSegmentDuration(length) || number+1 >= len(lines) {
		return false
	}
	_, valid := hlsSegmentNumber(lines[number+1])
	return valid
}

func matchesCopiedHLSSegment(timeline *copiedHLSTimeline, number, observed int, length float64) bool {
	return number == observed && observed < len(timeline.Keys) && length > 0 && matchesCopiedHLSLength(timeline, observed, length)
}
