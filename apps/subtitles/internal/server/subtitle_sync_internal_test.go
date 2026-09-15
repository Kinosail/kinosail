package server

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"
	"time"

	"github.com/zserge/govad"
)

func TestFindSubtitleAlignmentFindsOffsetAndFrameRate(t *testing.T) {
	t.Parallel()
	for name, expected := range map[string]subtitleAlignment{
		"offset":     {Scale: 1, Offset: 7 * time.Second},
		"frame rate": {Scale: 25.0 / 23.976, Offset: -3 * time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cues := syntheticSubtitleCues()
			audio := syntheticSpeech(cues, expected.Scale, expected.Offset)
			found, ok := findSubtitleAlignment(audio, cues)
			if !ok || math.Abs(found.Scale-expected.Scale) > 0.0001 || absDuration(found.Offset-expected.Offset) > 300*time.Millisecond {
				t.Fatalf("alignment = %#v, ok = %v", found, ok)
			}
		})
	}
}

func TestFindSubtitleAlignmentRejectsWeakEvidence(t *testing.T) {
	t.Parallel()
	audio := make([]float64, int(10*time.Minute/subtitleFrame))
	for index := range audio {
		audio[index] = float64((index*17)%31) / 100
	}
	if alignment, ok := findSubtitleAlignment(audio, syntheticSubtitleCues()); ok {
		t.Fatalf("weak alignment = %#v", alignment)
	}
}

func TestSynchronizeSubtitleAppliesSimpleModelsAndAbstains(t *testing.T) {
	t.Parallel()
	for name, expected := range map[string]struct {
		alignment subtitleAlignment
		method    string
		changed   bool
	}{
		"identity": {subtitleAlignment{Scale: 1}, "none", false},
		"offset":   {subtitleAlignment{Scale: 1, Offset: 7 * time.Second}, "global", true},
		"drift":    {subtitleAlignment{Scale: 25.0 / 23.976, Offset: -3 * time.Second}, "linear", true},
	} {
		t.Run(name, func(t *testing.T) {
			cues := syntheticSubtitleCues()
			input := cleanedSubtitle{Data: []byte("original subtitle"), Cues: cues, Cleanup: []string{"Normalized to UTF-8 SRT"}}
			audio := syntheticSpeech(cues, expected.alignment.Scale, expected.alignment.Offset)
			result, changed, err := synchronizeSubtitle(audio, input)
			if err != nil || changed != expected.changed || result.Synchronization != expected.method || len(result.Cues) != len(cues) || len(result.Data) == 0 {
				t.Fatalf("synchronized = %#v, changed = %v, err = %v", result, changed, err)
			}
		})
	}
	if _, changed, err := synchronizeSubtitle(make([]float64, 10), cleanedSubtitle{Cues: syntheticSubtitleCues()}); err == nil || changed {
		t.Fatalf("weak synchronization = changed %v, err %v", changed, err)
	}
}

func TestSubtitleAudioAnalysisHandlesFramesAndFailures(t *testing.T) {
	t.Parallel()
	frame := make([]byte, govad.SamplesPerFrame*2)
	frame[0], frame[1] = 0xff, 0x7f
	paddedFrame := make([]byte, len(frame)+1)
	copy(paddedFrame, frame)
	probabilities, err := processSubtitleAudio(bytes.NewReader(paddedFrame))
	if err != nil || len(probabilities) != 1 {
		t.Fatalf("probabilities = %d, err = %v", len(probabilities), err)
	}
	if _, err = processSubtitleAudio(iotest.ErrReader(errors.New("read failed"))); err == nil {
		t.Fatal("audio reader failure was accepted")
	}
	if _, err = newSubtitleSynchronizer("").speechProbabilities(t.Context(), "/media/movie.mkv"); err == nil {
		t.Fatal("missing analyzer was accepted")
	}
	ffmpeg := filepath.Join(t.TempDir(), "ffmpeg")
	if err = os.WriteFile(ffmpeg, []byte("#!/bin/sh\ndd if=/dev/zero bs=1024 count=1900 2>/dev/null\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(ffmpeg, 0o700); err != nil { //nolint:gosec // The local test fixture must be executable.
		t.Fatal(err)
	}
	if probabilities, err = newSubtitleSynchronizer(ffmpeg).speechProbabilities(t.Context(), "/media/movie.mkv"); err != nil || len(probabilities) < int(time.Minute/subtitleFrame) {
		t.Fatalf("local audio analysis = %d frames, %v", len(probabilities), err)
	}
	if _, err = newSubtitleSynchronizer("/usr/bin/false").speechProbabilities(t.Context(), "/media/movie.mkv"); err == nil {
		t.Fatal("failed audio analyzer was accepted")
	}
}

func syntheticSubtitleCues() []subtitleCue {
	cues := make([]subtitleCue, 0, 60)
	for index := 0; index < 60; index++ {
		start := time.Duration(15+index*13) * time.Second
		length := time.Duration(1+(index*7)%4) * time.Second
		cues = append(cues, subtitleCue{Start: start, End: start + length, Text: "Dialogue"})
	}
	return cues
}

func syntheticSpeech(cues []subtitleCue, scale float64, offset time.Duration) []float64 {
	audio := make([]float64, int(15*time.Minute/subtitleFrame))
	for index := range audio {
		audio[index] = 0.03 + float64((index*29)%17)/1000
	}
	for _, cue := range cues {
		start := int((time.Duration(float64(cue.Start)*scale) + offset) / subtitleFrame)
		end := int((time.Duration(float64(cue.End)*scale) + offset) / subtitleFrame)
		for index := max(0, start); index < min(len(audio), end); index++ {
			audio[index] = 0.88 + float64((index*11)%9)/100
		}
	}
	return audio
}
