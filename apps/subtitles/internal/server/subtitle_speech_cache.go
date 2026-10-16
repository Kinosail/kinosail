package server

import (
	"context"
	"errors"
	"os"
	"strconv"

	"github.com/MikeO7/kinosail/packages/library"
)

type subtitleSpeechCacheEntry struct {
	key       string
	reference subtitleAudioReference
}

type subtitleSpeechCall struct {
	done      chan struct{}
	reference subtitleAudioReference
	err       error
}

func (synchronizer *subtitleSynchronizer) speechForLanguage(ctx context.Context, item library.Item, language string) ([]float64, error) {
	reference, err := synchronizer.audioReference(ctx, item, language)
	return reference.Speech, err
}

func (synchronizer *subtitleSynchronizer) audioReference(ctx context.Context, item library.Item, language string) (subtitleAudioReference, error) {
	ctx, settled, admissionErr := synchronizer.admission.enter(ctx)
	if admissionErr != nil {
		return subtitleAudioReference{}, admissionErr
	}
	defer settled()
	if language != "" && !validLanguage(language) {
		return subtitleAudioReference{}, errors.New("subtitle language is invalid")
	}
	info, err := os.Stat(item.Path)
	if err != nil || !info.Mode().IsRegular() {
		return subtitleAudioReference{}, errors.New("video is unavailable")
	}
	stream, err := synchronizer.dialogueStream(ctx, item, language)
	if err != nil {
		return subtitleAudioReference{}, err
	}
	key := item.Path + "\x00" + strconv.FormatInt(info.Size(), 10) + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10) + "\x00" + stream
	return synchronizer.cachedReference(ctx, key, func(ctx context.Context) (subtitleAudioReference, error) {
		return synchronizer.extractAudio(ctx, item.Path, stream)
	})
}

func (synchronizer *subtitleSynchronizer) cachedReference(ctx context.Context, key string, analyze func(context.Context) (subtitleAudioReference, error)) (subtitleAudioReference, error) {
	synchronizer.mu.Lock()
	for _, cached := range synchronizer.cache {
		if cached.key == key {
			synchronizer.mu.Unlock()
			return cached.reference, nil
		}
	}
	if call := synchronizer.calls[key]; call != nil {
		synchronizer.mu.Unlock()
		select {
		case <-call.done:
			return call.reference, call.err
		case <-ctx.Done():
			return subtitleAudioReference{}, ctx.Err()
		}
	}
	call := &subtitleSpeechCall{done: make(chan struct{})}
	if synchronizer.calls == nil {
		synchronizer.calls = make(map[string]*subtitleSpeechCall)
	}
	synchronizer.calls[key] = call
	synchronizer.mu.Unlock()
	reference, found := synchronizer.loadSpeech(key)
	var err error
	if !found {
		reference, err = analyze(ctx)
		if err == nil {
			reference = quantizeSubtitleSpeech(reference)
			synchronizer.saveSpeech(key, reference)
		}
	}
	synchronizer.mu.Lock()
	defer synchronizer.mu.Unlock()
	delete(synchronizer.calls, key)
	call.reference, call.err = reference, err
	close(call.done)
	if err != nil {
		return subtitleAudioReference{}, err
	}
	synchronizer.cache = append([]subtitleSpeechCacheEntry{{key, reference}}, synchronizer.cache...)
	if len(synchronizer.cache) > 2 {
		synchronizer.cache = synchronizer.cache[:2]
	}
	return reference, nil
}

func subtitleDialogueTrack(tracks []AudioFacts, language string) (int, bool) {
	selected, best := -1, -1
	for _, track := range tracks {
		if track.Role != "main" || track.SourceIndex < 0 {
			continue
		}
		score := 0
		if track.Default {
			score++
		}
		if language != "" && subtitleLanguageMatches(language, track.Language) {
			score += 2
		}
		if score > best {
			selected, best = track.SourceIndex, score
		}
	}
	return selected, selected >= 0
}
