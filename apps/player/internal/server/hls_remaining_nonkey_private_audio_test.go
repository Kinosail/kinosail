package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Public preparation cannot expose the exact first private AAC payload or its
// metadata ambiguity. Actual generated fixtures protect that acquisition gap.
func TestRemainingNonKeyPrivateAudioAcquisition(t *testing.T) {
	if os.Getenv("KINOSAIL_COPIED_RECOVERY_MEDIA") != "1" {
		t.Skip("The pinned hosted job owns actual private audio acquisition")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("private-audio pinned FFmpeg missing")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	sources := remainingNonKeyCollectorSources(t, ctx, ffmpeg)
	for _, source := range sources {
		for _, offset := range []float64{12.5, 13.5, 18.2} {
			t.Run(fmt.Sprintf("%s-%.1f", filepath.Ext(source), offset), func(t *testing.T) {
				remainingNonKeyPrivateAudioCase(t, ctx, ffmpeg, source, offset)
			})
		}
	}
}

func remainingNonKeyPrivateAudioCase(t *testing.T, ctx context.Context, ffmpeg, source string, offset float64) {
	t.Helper()
	before := remainingNonKeyCollectorHash(t, source)
	directory := remainingNonKeyCollectorGeneratePrivate(t, ctx, ffmpeg, source, offset)
	initialization := remainingNonKeyCollectorRead(t, filepath.Join(directory, "init.mp4"), 2<<20)
	fragment := remainingNonKeyCollectorRead(t, filepath.Join(directory, "segment-00000.m4s"), 64<<20)
	inputInit, inputFirst := sha256.Sum256(initialization), sha256.Sum256(fragment)
	request, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	started := time.Now()
	facts, err := parseCopiedHLSPrivateAudio(request, initialization, fragment)
	elapsed := time.Since(started)
	if err != nil || facts == nil {
		t.Fatal("nonkey actual private AAC acquisition missing")
	}
	remainingNonKeyPrivateAudioCorrespondence(t, facts, source, offset, inputInit, inputFirst, elapsed)
	if sha256.Sum256(initialization) != inputInit || sha256.Sum256(fragment) != inputFirst ||
		remainingNonKeyCollectorHash(t, source) != before {
		t.Fatal("nonkey private audio acquisition changed inputs or source")
	}
	remainingNonKeyPrivateAudioRejects(t, ctx, initialization, fragment)
}

func remainingNonKeyPrivateAudioCorrespondence(t *testing.T, facts *copiedHLSPrivateAudioFacts, source string, offset float64, inputInit, inputFirst [32]byte, elapsed time.Duration) {
	t.Helper()
	expectedHash, expected := remainingNonKeyCollectorExpected(t, source, offset)
	if hex.EncodeToString(facts.FirstPacket[:]) != expectedHash || facts.OriginalMediaTime != expected[8] {
		t.Fatal("nonkey acquired private AAC differs from independently measured source packet and edit")
	}
	if facts.TrackID != 2 || facts.Timescale != 48000 || facts.Initialization != inputInit ||
		facts.First != inputFirst || elapsed > 2*time.Second {
		t.Fatal("nonkey private AAC track, asset binding or operation budget")
	}
	t.Logf("nonkey private-audio container=%s offset=%.1f elapsed_ns=%d track=%d timescale=%d init_sha=%x first_sha=%x first_aac_sha=%x original_edit=%d",
		filepath.Ext(source), offset, elapsed.Nanoseconds(), facts.TrackID, facts.Timescale,
		facts.Initialization, facts.First, facts.FirstPacket, facts.OriginalMediaTime)
}

func remainingNonKeyPrivateAudioRejects(t *testing.T, ctx context.Context, initialization, fragment []byte) {
	t.Helper()
	for _, name := range []string{"empty-init", "empty-first", "truncated-first", "duplicate-moov", "duplicate-mdat",
		"ambiguous-audio", "absent-audio-track", "empty-edit", "negative-edit", "edit-rate",
		"audio-sample-count", "audio-offset-header", "audio-offset-overflow", "audio-overlaps-video", "zero-audio-size"} {
		t.Run(name, func(t *testing.T) {
			initCopy, firstCopy := bytes.Clone(initialization), bytes.Clone(fragment)
			initCopy, firstCopy = remainingNonKeyPrivateAudioDamage(t, name, initCopy, firstCopy)
			if facts, err := parseCopiedHLSPrivateAudio(ctx, initCopy, firstCopy); err == nil || facts != nil {
				t.Fatal("nonkey damaged private AAC metadata acquired identity")
			}
		})
	}
	request, cancel := context.WithCancel(ctx)
	cancel()
	if facts, err := parseCopiedHLSPrivateAudio(request, initialization, fragment); err == nil || facts != nil {
		t.Fatal("nonkey canceled private AAC acquisition acquired identity")
	}
}

func TestRemainingNonKeyPrivateAudioInputBounds(t *testing.T) {
	for _, name := range []string{"init-byte-limit", "fragment-byte-limit"} {
		t.Run(name, func(t *testing.T) {
			initialization, fragment := []byte{1}, []byte{1}
			if name == "init-byte-limit" {
				initialization = make([]byte, (2<<20)+1)
			} else {
				fragment = make([]byte, (64<<20)+1)
			}
			if facts, err := parseCopiedHLSPrivateAudio(t.Context(), initialization, fragment); err == nil || facts != nil {
				t.Fatal("nonkey oversized private AAC input acquired identity")
			}
		})
	}
}
