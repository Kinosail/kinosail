package server

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"sync"
	"sync/atomic"

	"github.com/zserge/govad"
)

const (
	subtitleSpeechChunkFrames  = 1920
	subtitleSpeechWarmupFrames = 64
	subtitleSpeechFrameBytes   = govad.SamplesPerFrame * 2
	subtitleWaveformFrames     = 64
)

type subtitleSpeechChunk struct {
	index  int
	warmup int
	pcm    []byte
}

func processSubtitleAudio(reader io.Reader, workers int, slots chan struct{}) (subtitleAudioReference, error) {
	jobs := make(chan subtitleSpeechChunk, 1)
	results := make(map[int]subtitleAudioReference)
	var mu sync.Mutex
	var unavailable atomic.Bool
	var group sync.WaitGroup
	store := func(index int, result subtitleAudioReference) {
		mu.Lock()
		defer mu.Unlock()
		results[index] = result
	}
	for range max(workers, 1) {
		group.Go(func() {
			if !analyzeSubtitleSpeechChunks(jobs, slots, store) {
				unavailable.Store(true)
			}
		})
	}
	frames, readErr := readSubtitleSpeechChunks(reader, jobs)
	close(jobs)
	group.Wait()
	if unavailable.Load() {
		return subtitleAudioReference{}, errors.New("voice detector is unavailable")
	}
	if readErr != nil {
		return subtitleAudioReference{}, errors.New("audio analysis failed")
	}
	reference := subtitleAudioReference{Speech: make([]float64, 0, frames), Waveform: make([]float64, 0, frames/subtitleWaveformFrames+1)}
	for index := range len(results) {
		reference.Speech = append(reference.Speech, results[index].Speech...)
		reference.Waveform = append(reference.Waveform, results[index].Waveform...)
	}
	return reference, nil
}

func analyzeSubtitleSpeechChunks(jobs <-chan subtitleSpeechChunk, slots chan struct{}, store func(int, subtitleAudioReference)) bool {
	detector, err := govad.New()
	for chunk := range jobs {
		if err != nil {
			continue
		}
		if slots != nil {
			slots <- struct{}{}
		}
		result := analyzeSubtitleSpeechChunk(detector, chunk)
		if slots != nil {
			<-slots
		}
		store(chunk.index, result)
	}
	return err == nil
}

func readSubtitleSpeechChunks(reader io.Reader, jobs chan<- subtitleSpeechChunk) (int, error) {
	buffered := bufio.NewReaderSize(reader, 64<<10)
	maximumFrames := int(subtitleAudioLimit / subtitleFrame)
	frames, index := 0, 0
	var tail []byte
	for frames < maximumFrames {
		want := min(subtitleSpeechChunkFrames, maximumFrames-frames)
		pcm := make([]byte, len(tail)+want*subtitleSpeechFrameBytes)
		copy(pcm, tail)
		read, err := io.ReadFull(buffered, pcm[len(tail):])
		if whole := read / subtitleSpeechFrameBytes; whole > 0 {
			pcm = pcm[:len(tail)+whole*subtitleSpeechFrameBytes]
			jobs <- subtitleSpeechChunk{index: index, warmup: len(tail) / subtitleSpeechFrameBytes, pcm: pcm}
			index++
			frames += whole
			tail = append([]byte(nil), pcm[len(pcm)-min(len(pcm), subtitleSpeechWarmupFrames*subtitleSpeechFrameBytes):]...)
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return frames, nil
		}
		if err != nil {
			return frames, err
		}
	}
	return frames, nil
}

func analyzeSubtitleSpeechChunk(detector *govad.VAD, chunk subtitleSpeechChunk) subtitleAudioReference {
	detector.Reset()
	frames := len(chunk.pcm) / subtitleSpeechFrameBytes
	result := subtitleAudioReference{Speech: make([]float64, 0, frames-chunk.warmup), Waveform: make([]float64, 0, (frames-chunk.warmup)/subtitleWaveformFrames+1)}
	samples := make([]float32, govad.SamplesPerFrame)
	peak := 0.0
	for frame := range frames {
		input := chunk.pcm[frame*subtitleSpeechFrameBytes:]
		for position := range samples {
			sample := int32(binary.LittleEndian.Uint16(input[position*2:]))
			if sample >= 1<<15 {
				sample -= 1 << 16
			}
			samples[position] = float32(sample) / 32768
			if frame >= chunk.warmup {
				peak = max(peak, math.Abs(float64(samples[position])))
			}
		}
		probability := detector.Process(samples)
		if frame < chunk.warmup {
			continue
		}
		result.Speech = append(result.Speech, float64(probability))
		if len(result.Speech)%subtitleWaveformFrames == 0 {
			result.Waveform = append(result.Waveform, peak)
			peak = 0
		}
	}
	if len(result.Speech)%subtitleWaveformFrames != 0 {
		result.Waveform = append(result.Waveform, peak)
	}
	return result
}
