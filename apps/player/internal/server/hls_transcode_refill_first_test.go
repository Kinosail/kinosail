package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"testing"
)

// Certifying late recovery must not make the independently missing first
// fragment unrecoverable. Start it only after proving the late path left it absent.
func transcodeRefillRecoverFirst(t *testing.T, ctx context.Context, ffmpeg, ffprobe, base, owned string, original [][]byte, beforeOwned int) {
	t.Helper()
	refilled := bandwidthHTTPGET(t, ctx, base+"segment-00000.m4s")
	joined := copiedRecoveryJoinedCodecs(t, ctx, owned)
	if joined != beforeOwned+1 {
		t.Fatal("certified first fragment recovery did not join exactly one producer")
	}
	actual := [][]byte{original[0], refilled, original[2], original[3]}
	controlFrames := transcodeRefillDecode(t, ctx, ffmpeg, "first-original", original)
	actualFrames := transcodeRefillDecode(t, ctx, ffmpeg, "first-regenerated", actual)
	if controlFrames.Numerator != actualFrames.Numerator || controlFrames.Denominator != actualFrames.Denominator || !slices.Equal(controlFrames.Frames, actualFrames.Frames) {
		t.Error("certified first fragment changed decoded video clocks")
	}
	controlPCM, controlAudio := transcodeRefillAudio(t, ctx, ffmpeg, ffprobe, "first-original", original)
	actualPCM, actualAudio := transcodeRefillAudio(t, ctx, ffmpeg, ffprobe, "first-regenerated", actual)
	transcodeRefillCompareAudio(t, "first-regenerated", controlAudio, actualAudio)
	if !bytes.Equal(controlPCM, actualPCM) {
		t.Error("cold first-fragment reconstruction changed decoded audio content")
	}
	for index, name := range []string{"init.mp4", "segment-00001.m4s", "segment-00002.m4s"} {
		part := index
		if index > 0 {
			part++
		}
		if !bytes.Equal(original[part], bandwidthHTTPGET(t, ctx, base+name)) {
			t.Error("first fragment recovery changed retained metadata or media")
		}
	}
	final := copiedRecoveryJoinedCodecs(t, ctx, owned)
	transcodeRefillLog(t, map[string]any{"firstRecoveredAfterLate": true, "producers": final, "refilledSHA256": fmt.Sprintf("%x", sha256.Sum256(refilled)), "pcmExact": bytes.Equal(controlPCM, actualPCM)})
	if final != joined {
		t.Error("first-fragment retained reads triggered another producer")
	}
}
