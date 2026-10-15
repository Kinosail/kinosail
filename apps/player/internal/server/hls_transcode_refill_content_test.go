package server_test

import (
	"encoding/binary"
	"math"
	"testing"
)

func transcodeRefillContent(t *testing.T, original, mixed []byte, sequence, fragment []refillAudioPacket) {
	t.Helper()
	last := fragment[len(fragment)-1]
	joins := []int64{*fragment[0].PTS, *last.PTS + *last.Duration}
	for index, tick := range joins {
		center, radius := int(tick-*sequence[0].PTS)*4, 4800*4
		start, end := center-radius, center+radius
		if start < 0 || end+3072*4 > len(original) || end > len(mixed) {
			t.Fatal("audio content window is outside the public fixture")
		}
		control := original[start:end]
		correlation, residual := transcodeRefillWaveform(control, mixed[start:end])
		silenceCorrelation, silenceResidual := transcodeRefillWaveform(control, make([]byte, len(control)))
		shiftCorrelation, shiftResidual := transcodeRefillWaveform(control, original[start+3072*4:end+3072*4])
		silenceRejected := !transcodeRefillContentMatches(silenceCorrelation, silenceResidual)
		shiftRejected := !transcodeRefillContentMatches(shiftCorrelation, shiftResidual)
		transcodeRefillLog(t, map[string]any{
			"audioContentJoin": index, "clockTick": tick, "windowSamples": 9600,
			"correlation": correlation, "normalizedRMSError": residual,
			"silenceRejected": silenceRejected, "shiftRejected": shiftRejected,
		})
		if !silenceRejected || !shiftRejected {
			t.Fatal("nonperiodic content oracle accepted silence or a 64 ms shift")
		}
		if !transcodeRefillContentMatches(correlation, residual) {
			t.Errorf("public refill changed audio content at join %d: correlation=%.6f normalizedRMS=%.6f", index, correlation, residual)
		}
	}
}

func transcodeRefillContentMatches(correlation, residual float64) bool {
	return correlation >= 0.995 && residual <= 0.1
}

func transcodeRefillWaveform(original, actual []byte) (float64, float64) {
	var originalEnergy, actualEnergy, product, difference float64
	for index := 0; index < len(original); index += 2 {
		left := float64(int16(binary.LittleEndian.Uint16(original[index:]))) //nolint:gosec // Reinterpret the complete PCM two's-complement sample.
		right := float64(int16(binary.LittleEndian.Uint16(actual[index:])))  //nolint:gosec // Reinterpret the complete PCM two's-complement sample.
		originalEnergy += left * left
		actualEnergy += right * right
		product += left * right
		difference += (left - right) * (left - right)
	}
	return product / math.Sqrt(max(1, originalEnergy*actualEnergy)), math.Sqrt(difference / max(1, originalEnergy))
}
