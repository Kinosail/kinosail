package server

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"

	"github.com/zserge/govad"
)

func TestSynchronizeSubtitleRejectsWeakEvidence(t *testing.T) {
	t.Parallel()
	if _, changed, err := synchronizeSubtitle(nil, cleanedSubtitle{}); err == nil || changed {
		t.Fatalf("weak synchronization = changed %v, error %v", changed, err)
	}
}

func TestSynchronizeCandidateRejectsPreservedSubtitle(t *testing.T) {
	t.Parallel()
	attempt := &subtitleCandidateAttempt{provider: &subtitleProvider{sync: newSubtitleSynchronizer("")}}
	if _, err := attempt.synchronizeCandidate(cleanedSubtitle{}, subtitleDownloadCandidate{Preserve: true}); err == nil {
		t.Fatal("preserved subtitle was synchronized")
	}
}

func TestSynchronizeSubtitleCorrectsAConfidentOffset(t *testing.T) {
	t.Parallel()
	cues := syntheticSubtitleCues()
	input, err := subtitleDocument(cues, 0)
	if err != nil {
		t.Fatal(err)
	}
	audio := syntheticSpeech(cues, 1, 7*time.Second)
	aligned, changed, err := synchronizeSubtitle(audio, input)
	if err != nil || !changed || len(aligned.Cues) != len(cues) || absDuration(aligned.Cues[0].Start-cues[0].Start-7*time.Second) > 300*time.Millisecond {
		t.Fatalf("synchronization = first %#v, changed %v, error %v", aligned.Cues, changed, err)
	}
}

func TestSynchronizeSubtitleLeavesAnAlignedDocumentUnchanged(t *testing.T) {
	t.Parallel()
	cues := syntheticSubtitleCues()
	input, err := subtitleDocument(cues, 0)
	if err != nil {
		t.Fatal(err)
	}
	audio := syntheticSpeech(cues, 1, 0)
	aligned, changed, err := synchronizeSubtitle(audio, input)
	if err != nil || changed || !bytes.Equal(aligned.Data, input.Data) {
		t.Fatalf("aligned synchronization = changed %v, error %v", changed, err)
	}
}

func TestProcessSubtitleAudioHandlesFramesAndReadFailures(t *testing.T) {
	t.Parallel()
	input := bytes.Repeat([]byte{0xff}, govad.SamplesPerFrame*4)
	reference, err := processSubtitleAudioReference(bytes.NewReader(input))
	if err != nil || len(reference.Speech) != 2 {
		t.Fatalf("audio probabilities = %d, error %v", len(reference.Speech), err)
	}
	reference, err = processSubtitleAudioReference(bytes.NewReader(input[:len(input)-1]))
	if err != nil || len(reference.Speech) != 1 {
		t.Fatalf("partial audio probabilities = %d, error %v", len(reference.Speech), err)
	}
	want := errors.New("read failed")
	if _, err = processSubtitleAudioReference(errorReader{want}); err == nil {
		t.Fatal("audio read failure was accepted")
	}
}

func TestSpeechProbabilityAnalysisRejectsUnavailableTools(t *testing.T) {
	t.Parallel()
	if _, err := newSubtitleSynchronizer("").analyzeAudio(t.Context(), library.Item{Path: "film.mkv"}, ""); err == nil {
		t.Fatal("missing FFmpeg was accepted")
	}
	missing := filepath.Join(t.TempDir(), "missing-ffmpeg")
	if _, err := newSubtitleSynchronizer(missing).analyzeAudio(t.Context(), library.Item{Path: "film.mkv"}, ""); err == nil {
		t.Fatal("unavailable FFmpeg was accepted")
	}
}

func TestSpeechProbabilityAnalysisReadsBoundedPCM(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	success := filepath.Join(directory, "ffmpeg-success")
	if err := os.WriteFile(success, []byte("#!/bin/sh\ndd if=/dev/zero bs=1920000 count=1 2>/dev/null\n"), 0o700); err != nil { //nolint:gosec // The owner-only file is an executable test fixture.
		t.Fatal(err)
	}
	reference, err := newSubtitleSynchronizer(success).analyzeAudio(t.Context(), library.Item{Path: "film.mkv"}, "")
	if err != nil || len(reference.Speech) != int(time.Minute/subtitleFrame) {
		t.Fatalf("speech probabilities = %d, error %v", len(reference.Speech), err)
	}
	empty := filepath.Join(directory, "ffmpeg-empty")
	if err = os.WriteFile(empty, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil { //nolint:gosec // The owner-only file is an executable test fixture.
		t.Fatal(err)
	}
	if _, err = newSubtitleSynchronizer(empty).analyzeAudio(t.Context(), library.Item{Path: "film.mkv"}, ""); err == nil {
		t.Fatal("empty PCM analysis was accepted")
	}
}

func TestSubtitleSynchronizationMathRejectsDegenerateRanges(t *testing.T) {
	t.Parallel()
	audio := make([]float64, int(time.Minute/subtitleFrame))
	if alignment, ok := findSubtitleAlignment(audio, syntheticSubtitleCues()); ok {
		t.Fatalf("constant audio alignment = %#v", alignment)
	}
	if score, ok := subtitleCorrelation(make([]float64, 100), make([]float64, 100), 50); ok || score != 0 {
		t.Fatalf("short overlap correlation = %v, valid %v", score, ok)
	}
	if score, ok := subtitleCorrelation(make([]float64, 100), make([]float64, 100), 0); ok || score != 0 {
		t.Fatalf("constant correlation = %v, valid %v", score, ok)
	}
	if subtitleAbsInt(-3) != 3 {
		t.Fatal("negative subtitle distance was not normalized")
	}
}

type errorReader struct{ err error }

func (reader errorReader) Read([]byte) (int, error) { return 0, reader.err }

var _ io.Reader = errorReader{}
