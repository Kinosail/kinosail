package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
	"github.com/MikeO7/kinosail/packages/playback"
)

const transcodeHLSClockMarker = "#KINOSAIL-TRANSCODE-REFILL:"
const maximumTranscodeHLSClock = 8 << 20

type transcodeHLSClock struct {
	Version    int
	Policy     string
	Encoder    [32]byte
	Source     transcodeSourceClock
	Renditions map[string]transcodeRenditionClock
}

type transcodeHLSRecovery struct {
	clock     *transcodeHLSClock
	digest    [32]byte
	rendition string
	number    int
	original  hlsRecipe
}

type transcodeHLSRecoveryKey struct{}

// Completion has not yet released the generation owner. Missing or unsupported
// source proof leaves the ordinary streaming generation uncommitted.
func (manager *hlsManager) certifyTranscodeHLS(ctx context.Context, item library.Item, root string, options transcodeSettings, recipe hlsRecipe, qualities []PlaybackQuality, committed chan<- string) error {
	observation := ctx
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var digest [32]byte
	if err := manager.writeTranscodeHLSClock(ctx, item, root, options, recipe, qualities, &digest); err != nil {
		if errors.Is(err, mediaprobe.ErrProbeLifecycle) {
			return err
		}
		if observation.Err() == nil {
			slog.WarnContext(observation, "HLS transcode refill certification unavailable", "request_id", requestActivityID(observation), "playback_session", requestPlaybackSession(observation), "failure_class", "source-or-fragment-clock", "outcome", "ordinary-stream-retained", "budget_expired", ctx.Err() != nil)
		}
	} else {
		committed <- transcodeHLSClockMarker + hex.EncodeToString(digest[:])
		slog.DebugContext(ctx, "HLS transcode refill certified", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "renditions", len(qualities))
	}
	return nil
}

func (manager *hlsManager) writeTranscodeHLSClock(ctx context.Context, item library.Item, directory string, options transcodeSettings, recipe hlsRecipe, qualities []PlaybackQuality, digest *[32]byte) error {
	if options.Accelerator != "none" || options.Encoder != "libx264" {
		return errTranscodeHLSClock
	}
	source, err := manager.transcodeAudioSourceClock(ctx, item, recipe)
	if err != nil {
		return err
	}
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	encoder, err := copiedHLSCacheFile(root, ".encoder", 1024)
	if err != nil {
		return err
	}
	clock := transcodeHLSClock{Version: 1, Policy: options.Cache, Encoder: sha256.Sum256(encoder), Source: *source, Renditions: make(map[string]transcodeRenditionClock)}
	for _, quality := range qualities {
		media, err := root.OpenRoot(quality.Label)
		if err != nil {
			return errTranscodeHLSClock
		}
		measured, err := manager.certifyTranscodeRendition(ctx, media, source.Rate)
		current := manager.copiedHLSCanonicalGeneration(directory, quality.Label, root, media)
		_ = media.Close()
		if err != nil {
			return err
		}
		if !current {
			return errTranscodeHLSClock
		}
		clock.Renditions[quality.Label] = *measured
	}
	if !validTranscodeHLSClock(&clock) || manager.validateHLSPolicy(ctx, item, recipe, options.Cache) != nil {
		return errTranscodeHLSClock
	}
	data, err := json.Marshal(clock)
	if err != nil || len(data) > maximumTranscodeHLSClock {
		return errTranscodeHLSClock
	}
	if err := writeCopiedHLSMetadata(root, ".transcode-clock", data); err != nil {
		return err
	}
	*digest = sha256.Sum256(data)
	return nil
}

func (manager *hlsManager) certifyTranscodeRendition(ctx context.Context, root *os.Root, rate int64) (*transcodeRenditionClock, error) {
	manifest, err := copiedHLSCacheFile(root, "index.m3u8", maximumCopiedHLSTimelineBytes)
	if err != nil || !playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") {
		return nil, errTranscodeHLSClock
	}
	var names []string
	for _, line := range strings.Split(string(manifest), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if number, valid := hlsSegmentNumber(line); !valid || number != len(names) {
			return nil, errTranscodeHLSClock
		}
		names = append(names, line)
	}
	clock, err := manager.probeTranscodeFragments(ctx, root, names)
	if err != nil {
		return nil, err
	}
	if clock.Rate != rate || !validCopiedHLSAudioGrid(&copiedHLSAudioGrid{Rate: rate}) {
		return nil, errTranscodeHLSClock
	}
	if !transcodeManifestMatchesClocks(manifest, clock) {
		return nil, errTranscodeHLSClock
	}
	current, err := copiedHLSCacheFile(root, "index.m3u8", maximumCopiedHLSTimelineBytes)
	if err != nil || !bytes.Equal(manifest, current) {
		return nil, errTranscodeHLSClock
	}
	clock.Manifest = sha256.Sum256(manifest)
	return clock, nil
}

func transcodeHLSMasterMarker(manifest []byte) (string, error) {
	var found string
	for _, line := range strings.Split(string(manifest), "\n") {
		if !strings.HasPrefix(line, transcodeHLSClockMarker) {
			continue
		}
		value := strings.TrimPrefix(line, transcodeHLSClockMarker)
		decoded, err := hex.DecodeString(value)
		if found != "" || err != nil || len(decoded) != sha256.Size || hex.EncodeToString(decoded) != value {
			return "", errTranscodeHLSClock
		}
		found = line
	}
	return found, nil
}

func publishTranscodeHLSVariants(ctx context.Context, source, directory, policy string, qualities []PlaybackQuality, results <-chan error, committed <-chan string) error {
	marker := ""
	write := func(path string, manifest []byte) error {
		select {
		case marker = <-committed:
		default:
		}
		if marker != "" {
			manifest = append(manifest, []byte(marker+"\n")...)
		}
		return writeAtomicFile(path, manifest)
	}
	return playback.PublishVariantsObserved(ctx, source, directory, policy, qualities, results, 1, true, write, func() { hlsObservationFor(ctx).emit("media_ready", "") })
}

func (manager *hlsManager) readTranscodeHLSRecovery(ctx context.Context, directory, policy, rendition string, number int) (*transcodeHLSRecovery, error) {
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return nil, errTranscodeHLSClock
	}
	defer root.Close()
	master, err := copiedHLSCacheFile(root, "index.m3u8", maximumCopiedHLSTimelineBytes)
	if err != nil {
		return nil, errTranscodeHLSClock
	}
	marker, err := transcodeHLSMasterMarker(master)
	if err != nil || marker == "" {
		return nil, err
	}
	data, err := copiedHLSCacheFile(root, ".transcode-clock", maximumTranscodeHLSClock)
	digest := sha256.Sum256(data)
	if err != nil || marker != transcodeHLSClockMarker+hex.EncodeToString(digest[:]) {
		return nil, errTranscodeHLSClock
	}
	var clock transcodeHLSClock
	if httpguard.DecodeUniqueJSON(bytes.NewReader(data), maximumTranscodeHLSClock, &clock) != nil || clock.Policy != policy || !validTranscodeHLSClock(&clock) {
		return nil, errTranscodeHLSClock
	}
	encoder, err := copiedHLSCacheFile(root, ".encoder", 1024)
	if err != nil || sha256.Sum256(encoder) != clock.Encoder {
		return nil, errTranscodeHLSClock
	}
	selected, ok := clock.Renditions[rendition]
	if !ok || number < 0 || number >= len(selected.Cuts) || len(selected.Cuts) > 16384 || selected.VideoBase <= 0 || selected.Rate != clock.Source.Rate {
		return nil, errTranscodeHLSClock
	}
	media, err := root.OpenRoot(rendition)
	if err != nil {
		return nil, errTranscodeHLSClock
	}
	defer media.Close()
	initialization, err := copiedHLSAssetHash(ctx, media, "init.mp4", 2<<20)
	manifest, manifestErr := copiedHLSAssetHash(ctx, media, "index.m3u8", maximumCopiedHLSTimelineBytes)
	if err != nil || manifestErr != nil || initialization != selected.Initialization || manifest != selected.Manifest || !manager.copiedHLSCanonicalGeneration(directory, rendition, root, media) {
		return nil, errTranscodeHLSClock
	}
	return &transcodeHLSRecovery{clock: &clock, digest: digest, rendition: rendition, number: number}, nil
}

func transcodeFragmentName(number int) string {
	value := strconv.Itoa(number)
	return "segment-" + strings.Repeat("0", 5-len(value)) + value + ".m4s"
}
