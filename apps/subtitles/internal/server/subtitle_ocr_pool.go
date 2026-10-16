package server

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"sync"
)

const subtitleOCRRecognitionCache = 8192

type subtitleOCRResult struct {
	done  chan struct{}
	text  string
	words []subtitleDraftWord
	err   error
}

type subtitleOCRFrame struct {
	at     float64
	result *subtitleOCRResult
}

type subtitleOCRJob struct {
	image  []byte
	result *subtitleOCRResult
}

func readSubtitleOCRFrames(ctx context.Context, output io.Reader, times <-chan float64, executable, language string, duration float64) ([]subtitleCue, []subtitleDraftWord, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	workers := subtitleWorkerCount()
	jobs, frames := make(chan subtitleOCRJob, workers), make(chan subtitleOCRFrame, 4*workers)
	var group sync.WaitGroup
	for range workers {
		group.Go(func() {
			for job := range jobs {
				job.result.text, job.result.words, job.result.err = recognizeSubtitleImage(ctx, executable, language, job.image)
				close(job.result.done)
			}
		})
	}
	produced := make(chan error, 1)
	go func() {
		defer close(frames)
		defer close(jobs)
		produced <- produceSubtitleOCRFrames(ctx, output, times, jobs, frames)
	}()
	cues, words, err := collectSubtitleOCRFrames(ctx, frames, duration)
	if err != nil {
		cancel()
	}
	for range frames {
	}
	group.Wait()
	if produceErr := <-produced; err == nil && produceErr != nil {
		return nil, nil, produceErr
	}
	if err != nil {
		return nil, nil, err
	}
	return cues, words, nil
}

func produceSubtitleOCRFrames(ctx context.Context, output io.Reader, times <-chan float64, jobs chan<- subtitleOCRJob, frames chan<- subtitleOCRFrame) error {
	pixels := make([]byte, 1920*1080)
	recognized := make(map[[sha256.Size]byte]*subtitleOCRResult)
	last := -1.0
	for count := 0; ; count++ {
		_, err := io.ReadFull(output, pixels)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil || count >= 40000 {
			return errors.New("bitmap subtitle frame limit exceeded")
		}
		at, err := nextSubtitleFrameTime(ctx, times)
		if err != nil {
			return err
		}
		if at < last {
			return errors.New("bitmap subtitle times are out of order")
		}
		last = at
		image := subtitleOCRImage(pixels, 1920, 1080)
		digest := sha256.Sum256(image)
		result, seen := recognized[digest]
		if !seen {
			result = &subtitleOCRResult{done: make(chan struct{})}
			if len(recognized) >= subtitleOCRRecognitionCache {
				clear(recognized)
			}
			recognized[digest] = result
			if len(image) == 0 {
				result.words = []subtitleDraftWord{}
				close(result.done)
			} else if !sendSubtitleOCR(ctx, jobs, subtitleOCRJob{image, result}) {
				return ctx.Err()
			}
		}
		if !sendSubtitleOCR(ctx, frames, subtitleOCRFrame{at, result}) {
			return ctx.Err()
		}
	}
}

func sendSubtitleOCR[T any](ctx context.Context, channel chan<- T, value T) bool {
	select {
	case channel <- value:
		return true
	case <-ctx.Done():
		return false
	}
}

func collectSubtitleOCRFrames(ctx context.Context, frames <-chan subtitleOCRFrame, duration float64) ([]subtitleCue, []subtitleDraftWord, error) {
	cues, words := []subtitleCue{}, []subtitleDraftWord{}
	previous, previousWords, start := "", []subtitleDraftWord{}, 0.0
	for frame := range frames {
		select {
		case <-frame.result.done:
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
		if frame.result.err != nil {
			return nil, nil, frame.result.err
		}
		if frame.result.text != previous {
			cues, words = appendSubtitleOCRCue(cues, words, previousWords, previous, start, frame.at)
			previous, previousWords, start = frame.result.text, frame.result.words, frame.at
		}
		if len(cues) > 20000 || len(words) > 100000 {
			return nil, nil, errors.New("bitmap subtitle output exceeds the limit")
		}
	}
	cues, words = appendSubtitleOCRCue(cues, words, previousWords, previous, start, duration)
	if len(cues) > 20000 || len(words) > 100000 {
		return nil, nil, errors.New("bitmap subtitle output exceeds the limit")
	}
	return cues, words, nil
}
