package server

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/zserge/govad"
)

const (
	subtitleSpeechParallelTolerance     = 0.05
	subtitleSpeechParallelOutlierRate   = 0.05
	subtitleSpeechParallelDecisionFlips = 0.01
)

func syntheticSpeechPCM(duration time.Duration, seed uint64) []byte {
	random := rand.New(rand.NewPCG(seed, seed^0x5eed))
	samples := int(duration.Seconds() * 16000)
	output := make([]byte, samples*2)
	talking, remaining, pitch, phase := false, 0, 140.0, 0.0
	for position := range samples {
		if remaining == 0 {
			talking = !talking
			remaining = 16000 * (1 + random.IntN(3))
			pitch = 100 + random.Float64()*120
		}
		remaining--
		t := float64(position) / 16000
		value := (random.Float64() - 0.5) * 0.01
		if talking {
			phase += 2 * math.Pi * pitch * (1 + 0.05*math.Sin(2*math.Pi*3*t)) / 16000
			syllable := 0.5 + 0.5*math.Sin(2*math.Pi*4*t)
			voice := 0.0
			for harmonic := 1; harmonic <= 12; harmonic++ {
				formant := math.Exp(-math.Pow((float64(harmonic)*pitch-700)/500, 2)) + 0.5*math.Exp(-math.Pow((float64(harmonic)*pitch-1200)/600, 2))
				voice += formant * math.Sin(float64(harmonic)*phase) / float64(harmonic)
			}
			value += 0.4 * syllable * voice
		}
		binary.LittleEndian.PutUint16(output[position*2:], uint16(int16(max(-1, min(1, value))*32767)))
	}
	return output
}

func sequentialSpeechProbabilities(t testing.TB, pcm []byte) []float64 {
	t.Helper()
	detector, err := govad.New()
	if err != nil {
		t.Fatal(err)
	}
	samples := make([]float32, govad.SamplesPerFrame)
	probabilities := make([]float64, 0, len(pcm)/subtitleSpeechFrameBytes)
	for offset := 0; offset+subtitleSpeechFrameBytes <= len(pcm); offset += subtitleSpeechFrameBytes {
		for position := range samples {
			samples[position] = float32(int16(binary.LittleEndian.Uint16(pcm[offset+position*2:]))) / 32768
		}
		probabilities = append(probabilities, float64(detector.Process(samples)))
	}
	return probabilities
}

func TestParallelSpeechAnalysisMatchesSequentialAnalysis(t *testing.T) {
	t.Parallel()
	pcm := syntheticSpeechPCM(94*time.Second, 11)
	sequential := sequentialSpeechProbabilities(t, pcm)
	parallel, err := processSubtitleAudio(bytes.NewReader(pcm), 4, nil)
	if err != nil || len(parallel.Speech) != len(sequential) || len(parallel.Waveform) != (len(sequential)+subtitleWaveformFrames-1)/subtitleWaveformFrames {
		t.Fatalf("parallel analysis = %d frames, %d waveform blocks, %v; want %d frames", len(parallel.Speech), len(parallel.Waveform), err, len(sequential))
	}
	if len(sequential) <= subtitleSpeechChunkFrames+900 {
		t.Fatalf("fixture has %d frames; it must extend well past a chunk boundary", len(sequential))
	}
	outliers, flips, speech := 0, 0, 0
	for frame := range sequential {
		if sequential[frame] > 0.5 {
			speech++
		}
		if (sequential[frame] > 0.5) != (parallel.Speech[frame] > 0.5) {
			flips++
		}
		if math.Abs(sequential[frame]-parallel.Speech[frame]) > subtitleSpeechParallelTolerance {
			outliers++
		}
	}
	if speech < len(sequential)/5 || speech > len(sequential)*4/5 {
		t.Fatalf("fixture speech frames = %d of %d; want mixed speech and silence", speech, len(sequential))
	}
	if rate := float64(outliers) / float64(len(sequential)); rate > subtitleSpeechParallelOutlierRate {
		t.Fatalf("%d of %d frames differ by more than %.2f", outliers, len(sequential), subtitleSpeechParallelTolerance)
	}
	if rate := float64(flips) / float64(len(sequential)); rate > subtitleSpeechParallelDecisionFlips {
		t.Fatalf("%d of %d frames changed their speech decision", flips, len(sequential))
	}
}
