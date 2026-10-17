package server

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

const (
	subtitleAudioLimit = 8 * time.Hour
	subtitleSyncRange  = 120 * time.Second
	subtitleFrame      = 32 * time.Millisecond
	subtitleCoarseStep = 8
)

var errSubtitleTimingMismatch = errors.New("subtitle timing disagrees with the video")

var subtitleTimeScales = []float64{1, 25.0 / 23.976, 23.976 / 25.0}

type subtitleSynchronizer struct {
	ffmpeg    string
	probe     *mediaProbe
	slots     chan struct{}
	mu        sync.Mutex
	calls     map[string]*subtitleSpeechCall
	cache     []subtitleSpeechCacheEntry
	cacheDir  string
	admission *subtitleAdmission
}

type subtitleAlignment struct {
	Scale  float64
	Offset time.Duration
	Score  float64
	Margin float64
}

type subtitleAlignmentAnchor struct {
	At, Offset time.Duration
}

func newSubtitleSynchronizer(ffmpeg string) *subtitleSynchronizer {
	return &subtitleSynchronizer{ffmpeg: ffmpeg, slots: make(chan struct{}, subtitleWorkerCount())}
}

func synchronizeSubtitle(probabilities []float64, subtitle cleanedSubtitle) (cleanedSubtitle, bool, error) {
	alignment, ok := findSubtitleAlignment(probabilities, subtitle.Cues)
	if !ok {
		return cleanedSubtitle{}, false, errors.New("subtitle synchronization confidence is low")
	}
	anchors, piecewise := findPiecewiseSubtitleAlignment(probabilities, subtitle.Cues, alignment)
	if piecewise {
		if fitted, continuous := continuousSubtitleAlignment(alignment, anchors); continuous {
			alignment, piecewise = fitted, false
		} else if subtitleAbruptTimingChange(anchors) {
			return cleanedSubtitle{}, false, errSubtitleTimingMismatch
		}
	}
	unchanged := !piecewise && alignment.Scale == 1 && absDuration(alignment.Offset) < 150*time.Millisecond

	cues := make([]subtitleCue, len(subtitle.Cues))
	for index, cue := range subtitle.Cues {
		offset := alignment.Offset
		if piecewise {
			offset = subtitleInterpolatedOffset(time.Duration(float64(cue.Start)*alignment.Scale), anchors)
		}
		cues[index] = subtitleCue{
			Start: time.Duration(float64(cue.Start)*alignment.Scale) + offset,
			End:   time.Duration(float64(cue.End)*alignment.Scale) + offset,
			Text:  cue.Text,
		}
	}
	if !validateSubtitleRegions(probabilities, cues) {
		return cleanedSubtitle{}, false, errSubtitleTimingMismatch
	}
	if unchanged {
		subtitle.Synchronization = "none"
		subtitle.TimingEvidence = "audio"
		return subtitle, false, nil
	}

	aligned, err := subtitleDocument(cues, subtitle.Duplicates)
	aligned.Cleanup = subtitle.Cleanup
	aligned.Original = subtitle.Original
	aligned.TimingEvidence = "audio"
	aligned.Synchronization = "global"
	if alignment.Scale != 1 {
		aligned.Synchronization = "linear"
	}
	if piecewise {
		aligned.Synchronization = "piecewise"
	}
	return aligned, err == nil, err
}

func (synchronizer *subtitleSynchronizer) dialogueStream(ctx context.Context, item library.Item, language string) (string, error) {
	if synchronizer.probe == nil {
		return "0:a:0", nil
	}
	facts := synchronizer.probe.facts(ctx, item)
	if facts.Duration > subtitleAudioLimit.Seconds() {
		return "", errors.New("video exceeds synchronization limit")
	}
	selected, found := subtitleDialogueTrack(facts.AudioFacts, language)
	if !found {
		return "", errors.New("main dialogue audio is unavailable")
	}
	return "0:" + strconv.Itoa(selected), nil
}

func (synchronizer *subtitleSynchronizer) extractAudio(ctx context.Context, media, stream string) (subtitleAudioReference, error) {
	if synchronizer.ffmpeg == "" {
		return subtitleAudioReference{}, errors.New("subtitle synchronization is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, synchronizer.ffmpeg, "-v", "error", "-i", media, "-map", stream, "-vn", "-ac", "1", "-ar", "16000", "-t", strconv.Itoa(int(subtitleAudioLimit.Seconds())), "-f", "s16le", "pipe:1") //nolint:gosec // The executable and scanned media path are trusted server configuration.
	stdout, err := command.StdoutPipe()
	if err != nil {
		return subtitleAudioReference{}, errors.New("audio analysis failed")
	}
	if err = workload.StartBackground(command); err != nil {
		return subtitleAudioReference{}, errors.New("audio analysis failed")
	}
	reference, readErr := processSubtitleAudio(stdout, subtitleWorkerCount(), synchronizer.slots)
	if readErr != nil {
		cancel()
	}
	waitErr := command.Wait()
	if readErr != nil || waitErr != nil || len(reference.Speech) < int(time.Minute/subtitleFrame) {
		return subtitleAudioReference{}, errors.New("audio analysis failed")
	}
	return reference, nil
}

type subtitleAudioReference struct {
	Speech   []float64
	Waveform []float64
}
