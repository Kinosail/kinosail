package server

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/zserge/govad"
)

const (
	subtitleSpeechParallelDecisionFlips  = 0.03
	subtitleSpeechParallelMeanDifference = 0.03
)

type syntheticSpeechRandom uint64

func (state *syntheticSpeechRandom) next() uint64 {
	*state ^= *state << 13
	*state ^= *state >> 7
	*state ^= *state << 17
	return uint64(*state)
}

func (state *syntheticSpeechRandom) Float64() float64 { return float64(state.next()>>11) / (1 << 53) }

func (state *syntheticSpeechRandom) seconds() int { return []int{1, 2, 3}[state.next()%3] }

func syntheticSpeechPCM(duration time.Duration, seed uint64) ([]byte, []subtitleCue) {
	random := syntheticSpeechRandom(seed | 1)
	samples := int(duration.Seconds() * 16000)
	output := make([]int16, samples)
	cues := []subtitleCue{}
	talking, remaining, pitch, phase := false, 0, 140.0, 0.0
	for position := range samples {
		if remaining == 0 {
			talking = !talking
			remaining = 16000 * random.seconds()
			pitch = 100 + random.Float64()*120
			if talking {
				start := time.Duration(position) * time.Second / 16000
				cues = append(cues, subtitleCue{Start: start, End: start + time.Duration(remaining)*time.Second/16000, Text: "Line"})
			}
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
		output[position] = int16(max(-1, min(1, value)) * 32767)
	}
	var encoded bytes.Buffer
	_ = binary.Write(&encoded, binary.LittleEndian, output)
	return encoded.Bytes(), cues
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
			sample := int32(binary.LittleEndian.Uint16(pcm[offset+position*2:]))
			if sample >= 1<<15 {
				sample -= 1 << 16
			}
			samples[position] = float32(sample) / 32768
		}
		probabilities = append(probabilities, float64(detector.Process(samples)))
	}
	return probabilities
}

func TestParallelSpeechAnalysisMatchesSequentialAnalysis(t *testing.T) {
	t.Parallel()
	pcm, speech := syntheticSpeechPCM(94*time.Second, 11)
	sequential := sequentialSpeechProbabilities(t, pcm)
	parallel, err := processSubtitleAudio(bytes.NewReader(pcm), 4, nil)
	if err != nil || len(parallel.Speech) != len(sequential) || len(parallel.Waveform) != (len(sequential)+subtitleWaveformFrames-1)/subtitleWaveformFrames {
		t.Fatalf("parallel analysis = %d frames, %d waveform blocks, %v; want %d frames", len(parallel.Speech), len(parallel.Waveform), err, len(sequential))
	}
	if len(sequential) <= subtitleSpeechChunkFrames+900 {
		t.Fatalf("fixture has %d frames; it must extend well past a chunk boundary", len(sequential))
	}
	flips, difference := 0, 0.0
	for frame := range sequential {
		if (sequential[frame] > 0.5) != (parallel.Speech[frame] > 0.5) {
			flips++
		}
		difference += math.Abs(sequential[frame] - parallel.Speech[frame])
	}
	if rate := float64(flips) / float64(len(sequential)); rate > subtitleSpeechParallelDecisionFlips {
		t.Fatalf("%d of %d frames changed their speech decision", flips, len(sequential))
	}
	if mean := difference / float64(len(sequential)); mean > subtitleSpeechParallelMeanDifference {
		t.Fatalf("mean probability difference = %.4f; want at most %.4f", mean, subtitleSpeechParallelMeanDifference)
	}
	early := make([]subtitleCue, len(speech))
	for position, cue := range speech {
		early[position] = subtitleCue{Start: cue.Start - 1500*time.Millisecond, End: cue.End - 1500*time.Millisecond, Text: cue.Text}
	}
	want, wantOK := findSubtitleAlignment(sequential, early)
	got, gotOK := findSubtitleAlignment(parallel.Speech, early)
	t.Logf("decision changes %d/%d, mean difference %.4f, offsets %s and %s", flips, len(sequential), difference/float64(len(sequential)), got.Offset, want.Offset)
	if !wantOK || !gotOK || got.Scale != want.Scale || absDuration(got.Offset-want.Offset) > subtitleFrame || absDuration(got.Offset-1500*time.Millisecond) > 4*subtitleFrame {
		t.Fatalf("parallel alignment = %#v, %v; sequential alignment = %#v, %v; want offset near 1.5s", got, gotOK, want, wantOK)
	}
}
