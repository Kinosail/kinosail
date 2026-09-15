package server

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os/exec"
	"strconv"
	"time"

	"github.com/MikeO7/kinosail/packages/library"

	"github.com/zserge/govad"
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
	ffmpeg   string
	probe    *mediaProbe
	analysis chan struct{}
	cache    []subtitleSpeechCacheEntry
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
	return &subtitleSynchronizer{ffmpeg: ffmpeg, analysis: make(chan struct{}, 1)}
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

func (synchronizer *subtitleSynchronizer) speechProbabilities(ctx context.Context, media string) ([]float64, error) {
	return synchronizer.speechProbabilitiesFor(ctx, library.Item{ID: subtitleFingerprint([]byte(media))[:16], Path: media})
}

func (synchronizer *subtitleSynchronizer) speechProbabilitiesFor(ctx context.Context, item library.Item) ([]float64, error) {
	return synchronizer.analyzeSpeech(ctx, item, "")
}

func (synchronizer *subtitleSynchronizer) analyzeSpeech(ctx context.Context, item library.Item, language string) ([]float64, error) {
	reference, err := synchronizer.analyzeAudio(ctx, item, language)
	return reference.Speech, err
}

func (synchronizer *subtitleSynchronizer) analyzeAudio(ctx context.Context, item library.Item, language string) (subtitleAudioReference, error) {
	media := item.Path
	if synchronizer.ffmpeg == "" {
		return subtitleAudioReference{}, errors.New("subtitle synchronization is unavailable")
	}
	stream := "0:a:0"
	if synchronizer.probe != nil {
		facts := synchronizer.probe.facts(ctx, item)
		if facts.Duration > subtitleAudioLimit.Seconds() {
			return subtitleAudioReference{}, errors.New("video exceeds synchronization limit")
		}
		selected, found := subtitleDialogueTrack(facts.AudioFacts, language)
		if !found {
			return subtitleAudioReference{}, errors.New("main dialogue audio is unavailable")
		}
		stream = "0:" + strconv.Itoa(selected)
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, synchronizer.ffmpeg, "-v", "error", "-i", media, "-map", stream, "-vn", "-ac", "1", "-ar", "16000", "-t", strconv.Itoa(int(subtitleAudioLimit.Seconds())), "-f", "s16le", "pipe:1") //nolint:gosec // The executable and scanned media path are trusted server configuration.
	stdout, err := command.StdoutPipe()
	if err != nil {
		return subtitleAudioReference{}, errors.New("audio analysis failed")
	}
	if err = command.Start(); err != nil {
		return subtitleAudioReference{}, errors.New("audio analysis failed")
	}
	reference, readErr := processSubtitleAudioReference(stdout)
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

func processSubtitleAudio(reader io.Reader) ([]float64, error) {
	reference, err := processSubtitleAudioReference(reader)
	return reference.Speech, err
}

func processSubtitleAudioReference(reader io.Reader) (subtitleAudioReference, error) {
	detector, err := govad.New()
	if err != nil {
		return subtitleAudioReference{}, errors.New("voice detector is unavailable")
	}
	maximumFrames := int(subtitleAudioLimit / subtitleFrame)
	probabilities := make([]float64, 0, maximumFrames)
	waveform := make([]float64, 0, maximumFrames/64+1)
	peak := 0.0
	input := make([]byte, govad.SamplesPerFrame*2)
	samples := make([]float32, govad.SamplesPerFrame)
	buffered := bufio.NewReaderSize(reader, len(input)*4)
	for len(probabilities) < maximumFrames {
		if _, err = io.ReadFull(buffered, input); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return subtitleAudioReference{}, errors.New("audio analysis failed")
		}
		for index := range samples {
			sample := int32(binary.LittleEndian.Uint16(input[index*2:]))
			if sample >= 1<<15 {
				sample -= 1 << 16
			}
			samples[index] = float32(sample) / 32768
			peak = max(peak, math.Abs(float64(samples[index])))
		}
		probabilities = append(probabilities, float64(detector.Process(samples)))
		if len(probabilities)%64 == 0 {
			waveform = append(waveform, peak)
			peak = 0
		}
	}
	if len(probabilities)%64 != 0 {
		waveform = append(waveform, peak)
	}
	return subtitleAudioReference{Speech: probabilities, Waveform: waveform}, nil
}
