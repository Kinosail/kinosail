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

	"github.com/MikeO7/kinosail/packages/hlsmanifest"
	"github.com/MikeO7/kinosail/packages/httpguard"
)

const maximumCopiedHLSTimelineBytes = 256 << 10

func validCopiedHLSClockInput(ctx context.Context, rendition string, timeline *copiedHLSTimeline) bool {
	return ctx.Err() == nil && hlsFile(rendition) && strings.HasSuffix(rendition, "/index.m3u8") && validCopiedHLSTimeline(timeline) && timeline.Clock == nil
}

func validCopiedHLSTimeline(timeline *copiedHLSTimeline) bool {
	if !validCopiedHLSHeader(timeline) || !validCopiedHLSTimeBase(timeline) || !validCopiedHLSEnd(timeline.End) || !validCopiedHLSAudioGrid(timeline.AudioGrid) || !validCopiedHLSSourceAudio(timeline.AudioSource, timeline.End) {
		return false
	}
	for number := range timeline.Keys {
		if !validCopiedHLSCut(timeline, number) {
			return false
		}
	}
	return validCopiedHLSBoundClock(timeline)
}

func validCopiedHLSBoundClock(timeline *copiedHLSTimeline) bool {
	if timeline.Clock == nil {
		return timeline.AudioGrid == nil
	}
	if math.IsNaN(*timeline.Clock) || *timeline.Clock < 0 || *timeline.Clock > 1 {
		return false
	}
	if timeline.AudioGrid == nil {
		return true
	}
	_, consistent := copiedHLSAudioClockSamples(timeline)
	return timeline.AudioSource != nil && timeline.AudioGrid.Rate == timeline.AudioSource.Rate && consistent
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
	return manager.readCopiedHLSTimelineRoot(ctx, directory, policy, root)
}

func (manager *hlsManager) readCopiedHLSTimelineRoot(ctx context.Context, directory, policy string, root *os.Root) (*copiedHLSTimeline, error) {
	timeline, data, err := decodeCopiedHLSTimeline(root, policy)
	if err != nil || ctx.Err() != nil {
		return nil, errCopiedHLSIndex
	}
	if timeline.Clock != nil && manager.verifyCopiedHLSCertificate(ctx, directory, root, data, timeline) != nil {
		return nil, errCopiedHLSIndex
	}
	return timeline, nil
}

func decodeCopiedHLSTimeline(root *os.Root, policy string) (*copiedHLSTimeline, []byte, error) {
	binding, err := copiedHLSCacheFile(root, ".source", 16<<10)
	if err != nil || string(binding) != policy {
		return nil, nil, errCopiedHLSIndex
	}
	data, err := copiedHLSCacheFile(root, ".copy-timeline", maximumCopiedHLSTimelineBytes)
	var timeline copiedHLSTimeline
	if err != nil || httpguard.DecodeUniqueJSON(bytes.NewReader(data), maximumCopiedHLSTimelineBytes, &timeline) != nil || timeline.Policy != policy || !validCopiedHLSTimeline(&timeline) {
		return nil, nil, errCopiedHLSIndex
	}
	return &timeline, data, nil
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
	if name != ".copy-timeline" && name != ".copy-clock" && name != ".transcode-clock" && name != shortHEVCCompletionMarker {
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
	if !validCopiedHLSTimeline(timeline) || timeline.Clock == nil {
		return manifest, false
	}
	return hlsmanifest.CompleteCuts(manifest, copiedHLSCuts(timeline))
}

func copiedHLSCuts(timeline *copiedHLSTimeline) []float64 {
	cuts := make([]float64, len(timeline.Keys)+1)
	for number := range timeline.Keys {
		cuts[number] = timeline.point(number)
	}
	cuts[len(timeline.Keys)] = timeline.End
	return cuts
}

func copiedHLSTime(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func matchesCopiedHLSManifest(manifest []byte, timeline *copiedHLSTimeline) bool {
	return hlsmanifest.MatchesCuts(manifest, copiedHLSCuts(timeline))
}

func matchesCopiedHLSLength(timeline *copiedHLSTimeline, number int, length float64) bool {
	return hlsmanifest.MatchesCutLength(timeline.segmentEnd(number)-timeline.point(number), length, number+1 == len(timeline.Keys))
}

func validCopiedHLSManifestCut(lines []string, number int, length float64) bool {
	if invalidHLSSegmentDuration(length) || number+1 >= len(lines) {
		return false
	}
	_, valid := hlsSegmentNumber(lines[number+1])
	return valid
}
