package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
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
	remainingNonKeyPrivateAudioRejects(t, ctx, initialization, fragment)
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
			remainingNonKeyPrivateAudioRejectInput(t, ctx, initCopy, firstCopy, "nonkey damaged private AAC metadata acquired identity")
		})
	}
	request, cancel := context.WithCancel(ctx)
	cancel()
	remainingNonKeyPrivateAudioRejectInput(t, request, initialization, fragment, "nonkey canceled private AAC acquisition acquired identity")
}

func TestRemainingNonKeyPrivateAudioInputBounds(t *testing.T) {
	if os.Getenv("KINOSAIL_COPIED_RECOVERY_MEDIA") != "1" {
		t.Skip("The pinned hosted job owns otherwise-valid private audio byte bounds")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("private-audio byte-bound pinned FFmpeg missing")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	sources := remainingNonKeyCollectorSources(t, ctx, ffmpeg)
	directory := remainingNonKeyCollectorGeneratePrivate(t, ctx, ffmpeg, sources[0], 12.5)
	initialization := remainingNonKeyCollectorRead(t, filepath.Join(directory, "init.mp4"), 2<<20)
	fragment := remainingNonKeyCollectorRead(t, filepath.Join(directory, "segment-00000.m4s"), 64<<20)
	for _, limit := range []int{2 << 20, 64 << 20} {
		t.Run(fmt.Sprintf("cap-%d", limit), func(t *testing.T) {
			remainingNonKeyPrivateAudioByteBound(t, ctx, sources[0], initialization, fragment, limit)
		})
	}
}

func remainingNonKeyPrivateAudioByteBound(t *testing.T, ctx context.Context, source string, initialization, fragment []byte, limit int) {
	t.Helper()
	withinInit, withinFirst, overInit, overFirst := initialization, fragment, initialization, fragment
	if limit == 2<<20 {
		withinInit = remainingNonKeyPrivateAudioPad(t, initialization, limit)
		overInit = remainingNonKeyPrivateAudioPad(t, initialization, limit+1)
	} else {
		withinFirst = remainingNonKeyPrivateAudioPad(t, fragment, limit)
		overFirst = remainingNonKeyPrivateAudioPad(t, fragment, limit+1)
	}
	remainingNonKeyPrivateAudioRejectInput(t, ctx, overInit, overFirst, "nonkey oversized private AAC input acquired identity")
	beforeInit, beforeFirst := sha256.Sum256(withinInit), sha256.Sum256(withinFirst)
	request, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	started := time.Now()
	facts, err := parseCopiedHLSPrivateAudio(request, withinInit, withinFirst)
	if err != nil || facts == nil {
		t.Fatal("nonkey otherwise-valid private AAC at its original byte cap was rejected")
	}
	if sha256.Sum256(withinInit) != beforeInit || sha256.Sum256(withinFirst) != beforeFirst {
		t.Fatal("nonkey accepted byte-cap private AAC snapshot was mutated")
	}
	remainingNonKeyPrivateAudioCorrespondence(t, facts, source, 12.5, beforeInit, beforeFirst, time.Since(started))
}

func remainingNonKeyPrivateAudioPad(t *testing.T, data []byte, size int) []byte {
	t.Helper()
	if size < len(data)+16 || size > (64<<20)+1 {
		t.Fatal("private AAC independent padding bound")
	}
	padding := make([]byte, size-len(data))
	binary.BigEndian.PutUint32(padding[:4], 1)
	copy(padding[4:8], "free")
	binary.BigEndian.PutUint64(padding[8:16], uint64(len(padding)))
	return append(bytes.Clone(data), padding...)
}

func remainingNonKeyPrivateAudioRejectInput(t *testing.T, ctx context.Context, initialization, fragment []byte, failure string) {
	t.Helper()
	beforeInit, beforeFirst := sha256.Sum256(initialization), sha256.Sum256(fragment)
	if facts, err := parseCopiedHLSPrivateAudio(ctx, initialization, fragment); err == nil || facts != nil {
		t.Fatal(failure)
	}
	if sha256.Sum256(initialization) != beforeInit || sha256.Sum256(fragment) != beforeFirst {
		t.Fatal("nonkey rejected private AAC snapshot was mutated")
	}
}
