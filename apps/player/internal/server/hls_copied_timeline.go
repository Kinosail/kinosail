package server

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/playback"
)

const maximumCopiedHLSTimelineBytes = 256 << 10

func validCopiedHLSTimeline(timeline *copiedHLSTimeline) bool {
	if timeline == nil || timeline.Strategy != "h264-idr-keys-1" || timeline.Policy == "" || len(timeline.Policy) > 16<<10 ||
		len(timeline.Keys) == 0 || len(timeline.Keys) > maximumCopiedHLSKeys ||
		timeline.Numerator <= 0 || timeline.Denominator <= 0 ||
		timeline.TimeBase != float64(timeline.Numerator)/float64(timeline.Denominator) || timeline.TimeBase <= 0 || timeline.TimeBase > 0.001 || math.IsNaN(timeline.TimeBase) ||
		timeline.End <= 0 || timeline.End > 7*24*60*60 || math.IsNaN(timeline.End) {
		return false
	}
	for number := range timeline.Keys {
		point := timeline.point(number)
		if point < 0 || point >= timeline.End || number > 0 && point-timeline.point(number-1) < 0.25 {
			return false
		}
		next := timeline.End
		if number+1 < len(timeline.Keys) {
			next = timeline.point(number + 1)
		}
		if invalidHLSSegmentDuration(next - point) {
			return false
		}
	}
	return timeline.Clock == nil || !math.IsNaN(*timeline.Clock) && *timeline.Clock >= 0 && *timeline.Clock <= 1
}

func copiedHLSCacheFile(root *os.Root, name string, limit int64) ([]byte, error) {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		return nil, errCopiedHLSIndex
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errCopiedHLSIndex
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errCopiedHLSIndex
	}
	return data, nil
}

func (manager *hlsManager) readCopiedHLSTimeline(directory, policy string) (*copiedHLSTimeline, error) {
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
	if err != nil || json.Unmarshal(data, &timeline) != nil || timeline.Policy != policy || !validCopiedHLSTimeline(&timeline) {
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
	file, err := root.OpenFile(".copy-timeline.pending", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer func() {
		_ = file.Close()
		_ = root.Remove(".copy-timeline.pending")
	}()
	_, err = file.Write(data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return errCopiedHLSIndex
	}
	return root.Rename(".copy-timeline.pending", ".copy-timeline")
}

func copiedHLSManifest(manifest []byte, timeline *copiedHLSTimeline) ([]byte, bool) {
	if !validCopiedHLSTimeline(timeline) || timeline.Clock == nil ||
		!playback.PlaylistHas(manifest, "#EXT-X-PLAYLIST-TYPE:EVENT") ||
		!bytes.Contains(manifest, []byte("#EXT-X-MAP:URI=\"init.mp4\"")) {
		return manifest, false
	}
	length, observed := 0.0, 0
	for _, line := range strings.Split(string(manifest), "\n") {
		if strings.HasPrefix(line, "#EXTINF:") {
			value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
			var err error
			length, err = strconv.ParseFloat(value, 64)
			if err != nil {
				return manifest, false
			}
		}
		if number, ok := hlsSegmentNumber(line); ok {
			if number != observed || observed >= len(timeline.Keys) || length <= 0 {
				return manifest, false
			}
			end := timeline.End
			if observed+1 < len(timeline.Keys) {
				end = timeline.point(observed + 1)
			}
			tolerance := 0.002
			if observed+1 == len(timeline.Keys) {
				tolerance = min(1, max(0.1, (end-timeline.point(observed))*0.05)) // Demux duration ticks can round down across the last GOP.
			}
			if math.Abs(length-(end-timeline.point(observed))) > tolerance {
				return manifest, false
			}
			observed++
			length = 0
		}
	}
	if observed == 0 || playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") && observed != len(timeline.Keys) {
		return manifest, false
	}
	target := 1.0
	for number := range timeline.Keys {
		end := timeline.End
		if number+1 < len(timeline.Keys) {
			end = timeline.point(number + 1)
		}
		target = max(target, math.Ceil(end-timeline.point(number)))
	}
	var output strings.Builder
	output.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:" + strconv.FormatFloat(target, 'f', 0, 64) +
		"\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-MAP:URI=\"init.mp4\"\n")
	for number := range timeline.Keys {
		end := timeline.End
		if number+1 < len(timeline.Keys) {
			end = timeline.point(number + 1)
		}
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

// A genuine final segment can have a shortened mux EXTINF after demux packet
// durations round down. Correct only that existing URI against known duration.
func completedCopiedHLSManifest(manifest []byte, duration float64) []byte {
	lines := strings.Split(string(manifest), "\n")
	sum, last, final := 0.0, 0.0, -1
	for number, line := range lines {
		if !strings.HasPrefix(line, "#EXTINF:") {
			continue
		}
		value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
		length, err := strconv.ParseFloat(value, 64)
		if err != nil || invalidHLSSegmentDuration(length) || number+1 >= len(lines) {
			return manifest
		}
		if _, valid := hlsSegmentNumber(lines[number+1]); !valid {
			return manifest
		}
		sum, last, final = sum+length, length, number
	}
	if final < 0 || duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) || duration > 7*24*60*60 {
		return manifest
	}
	corrected := duration - (sum - last)
	if invalidHLSSegmentDuration(corrected) || math.Abs(corrected-last) > min(1, max(0.1, last*0.05)) {
		return bytes.Replace(manifest, []byte("#EXT-X-PLAYLIST-TYPE:EVENT"), []byte("#EXT-X-PLAYLIST-TYPE:VOD"), 1)
	}
	lines[final] = "#EXTINF:" + copiedHLSTime(corrected) + ","
	return bytes.Replace([]byte(strings.Join(lines, "\n")), []byte("#EXT-X-PLAYLIST-TYPE:EVENT"), []byte("#EXT-X-PLAYLIST-TYPE:VOD"), 1)
}
