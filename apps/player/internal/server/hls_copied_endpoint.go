package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/playback"
)

type copiedHLSEndpointOutput struct{ copiedHLSProbeOutput }

type copiedHLSClockGeneration struct {
	assets [4]os.FileInfo
	hashes [4][32]byte
}

func copiedHLSClockPendingSnapshot(ctx context.Context, root, media *os.Root, policy string, timeline *copiedHLSTimeline) (copiedHLSClockGeneration, error) {
	before, err := copiedHLSClockSnapshot(ctx, root, media)
	data, marshalErr := json.Marshal(timeline)
	if err != nil || marshalErr != nil || before.hashes[0] != sha256.Sum256([]byte(policy)) || before.hashes[1] != sha256.Sum256(data) {
		return before, errCopiedHLSIndex
	}
	return before, nil
}

func copiedHLSClockSnapshot(ctx context.Context, root, media *os.Root) (copiedHLSClockGeneration, error) {
	var result copiedHLSClockGeneration
	for number, name := range []string{".source", ".copy-timeline", "init.mp4", "segment-00000.m4s"} {
		owner, limit := root, int64(maximumCopiedHLSTimelineBytes)
		if number == 0 {
			limit = 16 << 10
		}
		if number >= 2 {
			owner, limit = media, 64<<20
		}
		if number == 2 {
			limit = 2 << 20
		}
		info, err := owner.Lstat(name)
		if err != nil {
			return result, errCopiedHLSIndex
		}
		hash, err := copiedHLSAssetHash(ctx, owner, name, limit)
		if err != nil {
			return result, err
		}
		result.assets[number], result.hashes[number] = info, hash
	}
	return result, nil
}

func (manager *hlsManager) copiedHLSCanonicalGeneration(directory, name string, root, media *os.Root) bool {
	canonical, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return false
	}
	defer canonical.Close()
	first, err := root.Stat(".")
	second, currentErr := canonical.Stat(".")
	if err != nil || currentErr != nil || !os.SameFile(first, second) {
		return false
	}
	first, err = media.Stat(".")
	second, currentErr = canonical.Lstat(name)
	return err == nil && currentErr == nil && second.IsDir() && os.SameFile(first, second)
}

func (manager *hlsManager) sameCopiedHLSClockGeneration(ctx context.Context, directory, name string, root, media *os.Root, before copiedHLSClockGeneration) bool {
	if ctx.Err() != nil || !manager.copiedHLSCanonicalGeneration(directory, name, root, media) {
		return false
	}
	after, err := copiedHLSClockSnapshot(ctx, root, media)
	if err != nil || before.hashes != after.hashes {
		return false
	}
	for number := range before.assets {
		if !sameCopiedHLSFile(before.assets[number], after.assets[number]) {
			return false
		}
	}
	return true
}

func (output *copiedHLSEndpointOutput) Write(data []byte) (int, error) {
	if output.Len()+len(data) > 1<<20 {
		return 0, errCopiedHLSIndex
	}
	return output.Buffer.Write(data)
}

// Read the actual generated final fragment to EOF. Container format duration
// alone never authorizes extra media or a new URI.
func (manager *hlsManager) completedCopiedHLSEndpoint(ctx context.Context, root *os.Root, manifest []byte) (float64, error) {
	last := copiedHLSLastSegment(manifest)
	if last == "" || !playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") || ctx.Err() != nil {
		return 0, errCopiedHLSIndex
	}
	clock, err := manager.measureCopiedHLSClock(ctx, root)
	if err != nil {
		return 0, err
	}
	initialization, err := copiedHLSCacheFile(root, "init.mp4", 2<<20)
	if err != nil {
		return 0, err
	}
	fragment, err := copiedHLSCacheFile(root, last, 64<<20)
	if err != nil {
		return 0, err
	}
	output := copiedHLSEndpointOutput{}
	//nolint:gosec // Probe is installation config and input is bounded generated media.
	command := exec.CommandContext(ctx, manager.probe.executable, "-v", "error", "-threads", "1", "-select_streams", "v:0",
		"-show_packets", "-show_entries", "packet=pts_time,duration_time", "-of", "json", "pipe:0")
	command.Stdin = io.MultiReader(bytes.NewReader(initialization), bytes.NewReader(fragment))
	command.Stdout = &output
	if command.Run() != nil {
		return 0, errCopiedHLSIndex
	}
	end, err := decodeCopiedHLSEndpoint(output.Bytes())
	return end - clock, err
}

func (manager *hlsManager) completedCopiedHLSProjection(ctx context.Context, directory, policy string, manifest []byte) []byte {
	end, err := manager.copiedHLSEndpoint(ctx, directory, policy, manifest)
	if err != nil {
		return bytes.Replace(manifest, []byte("#EXT-X-PLAYLIST-TYPE:EVENT"), []byte("#EXT-X-PLAYLIST-TYPE:VOD"), 1)
	}
	return completedCopiedHLSManifest(manifest, end)
}

func copiedHLSLastSegment(manifest []byte) string {
	last := ""
	for _, line := range strings.Split(string(manifest), "\n") {
		if _, valid := hlsSegmentNumber(line); valid {
			last = line
		}
	}
	return last
}

func decodeCopiedHLSEndpoint(data []byte) (float64, error) {
	var facts struct {
		Packets []struct {
			PTS      string `json:"pts_time"`
			Duration string `json:"duration_time"`
		} `json:"packets"`
	}
	if json.Unmarshal(data, &facts) != nil || len(facts.Packets) == 0 || len(facts.Packets) > 4096 {
		return 0, errCopiedHLSIndex
	}
	end := 0.0
	for _, packet := range facts.Packets {
		packetEnd, err := copiedHLSPacketEnd(packet.PTS, packet.Duration)
		if err != nil {
			return 0, err
		}
		end = max(end, packetEnd)
	}
	return end, nil
}

func copiedHLSPacketEnd(ptsValue, durationValue string) (float64, error) {
	pts, ptsErr := strconv.ParseFloat(ptsValue, 64)
	duration, durationErr := strconv.ParseFloat(durationValue, 64)
	if ptsErr != nil || durationErr != nil || math.IsNaN(pts) || math.IsNaN(duration) ||
		math.IsInf(pts, 0) || math.IsInf(duration, 0) || duration <= 0 {
		return 0, errCopiedHLSIndex
	}
	return pts + duration, nil
}

// A genuine final segment can have a shortened mux EXTINF after demux packet
// durations round down. Correct only that existing URI against known duration.
func completedCopiedHLSManifest(manifest []byte, duration float64) []byte {
	lines := strings.Split(string(manifest), "\n")
	sum, last, final, valid := copiedHLSManifestExtents(lines)
	if !valid || !validCopiedHLSEnd(duration) {
		return manifest
	}
	corrected := duration - (sum - last)
	if invalidHLSSegmentDuration(corrected) || math.Abs(corrected-last) > min(1, max(0.1, last*0.05)) {
		return bytes.Replace(manifest, []byte("#EXT-X-PLAYLIST-TYPE:EVENT"), []byte("#EXT-X-PLAYLIST-TYPE:VOD"), 1)
	}
	target, targetErr := copiedHLSManifestTarget(lines)
	serialized, err := strconv.ParseFloat(copiedHLSTime(corrected), 64)
	if err != nil || targetErr != nil || math.Round(serialized) > target {
		return nil
	}
	lines[final] = "#EXTINF:" + copiedHLSTime(corrected) + ","
	return bytes.Replace([]byte(strings.Join(lines, "\n")), []byte("#EXT-X-PLAYLIST-TYPE:EVENT"), []byte("#EXT-X-PLAYLIST-TYPE:VOD"), 1)
}

func copiedHLSManifestExtents(lines []string) (float64, float64, int, bool) {
	sum, last, final := 0.0, 0.0, -1
	for number, line := range lines {
		if !strings.HasPrefix(line, "#EXTINF:") {
			continue
		}
		value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
		length, err := strconv.ParseFloat(value, 64)
		if err != nil || !validCopiedHLSManifestCut(lines, number, length) {
			return 0, 0, -1, false
		}
		sum, last, final = sum+length, length, number
	}
	return sum, last, final, final >= 0
}

func copiedHLSManifestTarget(lines []string) (float64, error) {
	target, seen := 0.0, false
	for _, line := range lines {
		if !strings.HasPrefix(line, "#EXT-X-TARGETDURATION:") {
			continue
		}
		value, err := copiedHLSTarget(strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:"))
		if seen || err != nil {
			return 0, errCopiedHLSIndex
		}
		target, seen = value, true
	}
	if !seen {
		return 0, errCopiedHLSIndex
	}
	return target, nil
}

func copiedHLSTarget(value string) (float64, error) {
	if len(value) == 0 || len(value) > 20 {
		return 0, errCopiedHLSIndex
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return 0, errCopiedHLSIndex
		}
	}
	integer, err := strconv.ParseUint(value, 10, 64)
	if err != nil || integer == 0 || integer > 7*24*60*60 {
		return 0, errCopiedHLSIndex
	}
	return float64(integer), nil
}

func copiedHLSBoundMetadata(root *os.Root, timeline, certificate []byte, policy string) bool {
	for number, name := range []string{".source", ".copy-timeline", ".copy-clock"} {
		expected, limit := []byte(policy), int64(16<<10)
		if number == 1 {
			expected, limit = timeline, maximumCopiedHLSTimelineBytes
		}
		if number == 2 {
			expected, limit = certificate, 4096
		}
		current, err := copiedHLSCacheFile(root, name, limit)
		if err != nil || !bytes.Equal(current, expected) {
			return false
		}
	}
	return true
}
