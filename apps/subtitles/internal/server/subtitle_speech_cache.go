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

func (synchronizer *subtitleSynchronizer) speechForLanguage(ctx context.Context, item library.Item, language string) ([]float64, error) {
	reference, err := synchronizer.audioReference(ctx, item, language)
	return reference.Speech, err
}

func (synchronizer *subtitleSynchronizer) audioReference(ctx context.Context, item library.Item, language string) (subtitleAudioReference, error) {
	if language != "" && !validLanguage(language) {
		return subtitleAudioReference{}, errors.New("subtitle language is invalid")
	}
	info, err := os.Stat(item.Path)
	if err != nil || !info.Mode().IsRegular() {
		return subtitleAudioReference{}, errors.New("video is unavailable")
	}
	key := item.Path + "\x00" + strconv.FormatInt(info.Size(), 10) + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10) + ":" + language
	select {
	case synchronizer.analysis <- struct{}{}:
		defer func() { <-synchronizer.analysis }()
	case <-ctx.Done():
		return subtitleAudioReference{}, ctx.Err()
	}
	for _, cached := range synchronizer.cache {
		if cached.key == key {
			return cached.reference, nil
		}
	}
	reference, err := synchronizer.analyzeAudio(ctx, item, language)
	if err != nil {
		return subtitleAudioReference{}, err
	}
	// Two full-length references bound memory and are shared by search, preview,
	// and retries. File changes invalidate the key automatically.
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
