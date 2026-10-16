package server

import (
	"bytes"
	"testing"
	"time"
)

func BenchmarkSubtitleSpeechAnalysisTenMinutes(b *testing.B) {
	pcm := syntheticSpeechPCM(10*time.Minute+30*time.Second, 3)
	b.SetBytes(int64(len(pcm)))
	for b.Loop() {
		reference, err := processSubtitleAudioReference(bytes.NewReader(pcm))
		if err != nil || len(reference.Speech) != len(pcm)/1024 {
			b.Fatalf("frames=%d error=%v", len(reference.Speech), err)
		}
	}
}
