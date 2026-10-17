package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
)

func (manager *subtitleManager) ocrLanguages() *ocrLanguageStore {
	manager.ocrOnce.Do(func() { manager.ocr = newOCRLanguageStore(manager.probe.cacheDir) })
	return manager.ocr
}

func (manager *subtitleManager) imageSubtitleFacts(media probeResult, language string) (SubtitleFacts, bool) {
	best, score := SubtitleFacts{}, -1
	preference := manager.preference()
	for _, track := range media.SubtitleFacts {
		if track.Text || track.External || track.Forced || track.SourceIndex < 0 || !mediaprobe.ImageSubtitle(track.Codec) || !subtitleLanguageMatches(language, track.Language) {
			continue
		}
		trackScore := 0
		if (preference == "sdh") == (track.Role == "captions") {
			trackScore += 2
		}
		if track.Default {
			trackScore++
		}
		if trackScore > score {
			best, score = track, trackScore
		}
	}
	return best, score >= 0 && manager.probe.cacheDir != "" && manager.ocrLanguages().supported(language)
}

func (manager *subtitleManager) imageSubtitle(ctx context.Context, item library.Item, language string) ([]byte, SubtitleFacts, bool) {
	if !manager.embeddedReady() {
		return nil, SubtitleFacts{}, false
	}
	track, found := manager.imageSubtitleFacts(manager.probe.facts(ctx, item), language)
	if !found {
		return nil, SubtitleFacts{}, false
	}
	if data, err := manager.probe.core.ConvertedSubtitle(item, track.SourceIndex, manager.probe.cacheDir); err == nil {
		return data, track, true
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	tesseract := firstNonempty(manager.settings.configuration().String("binaries.tesseract"), "tesseract")
	data, err := manager.ocrLanguages().ensure(ctx, tesseract, language)
	if err != nil {
		return nil, SubtitleFacts{}, false
	}
	started := time.Now()
	options := mediaprobe.ImageTextOptions{FFmpeg: manager.probe.ffmpeg, Tesseract: tesseract, CacheDir: manager.probe.cacheDir, Language: language, TessdataDir: data}
	if err = manager.probe.core.ConvertImageSubtitles(ctx, item, []int{track.SourceIndex}, options); err != nil {
		if ctx.Err() == nil {
			slog.WarnContext(ctx, "image subtitle text recognition failed", "operation", "subtitles.ocr", "item_id", item.ID, "language", language, "error", err)
		}
		return nil, SubtitleFacts{}, false
	}
	converted, err := manager.probe.core.ConvertedSubtitle(item, track.SourceIndex, manager.probe.cacheDir)
	if err != nil {
		return nil, SubtitleFacts{}, false
	}
	slog.InfoContext(ctx, "image subtitle converted to text", "operation", "subtitles.ocr", "item_id", item.ID, "language", language, "duration_ms", time.Since(started).Milliseconds())
	return converted, track, true
}
