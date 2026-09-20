package server

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
)

type subtitleDraftTool struct {
	executable, model, architecture, language string
	stream                                    int
}

func (current *subtitleDraft) cancelRequest(id string, input subtitleDraftInput) (subtitleDraft, int, error) {
	if current.ID != input.DraftID || current.Item != id || current.Language != input.Language || current.Method != input.Method {
		return subtitleDraft{}, http.StatusConflict, errors.New("local draft is no longer current")
	}
	if current.cancel != nil {
		current.cancel()
		current.State = "canceling"
		current.Message = "Stopping local generation…"
	}
	return *current, http.StatusOK, nil
}

func selectSubtitleDraftTool(config configuration.Snapshot, facts probeResult, input subtitleDraftInput) (subtitleDraftTool, error) {
	if input.Method == "ocr" {
		return selectSubtitleOCRTool(config, facts, input.Language)
	}
	return selectSubtitleTranscriptionTool(config, facts, input.Language)
}

func selectSubtitleOCRTool(config configuration.Snapshot, facts probeResult, language string) (subtitleDraftTool, error) {
	tool := subtitleDraftTool{executable: firstNonempty(config.String("binaries.tesseract"), "tesseract"), language: subtitleOCRLanguage(language), stream: -1}
	for _, track := range facts.SubtitleFacts {
		if !track.Text && !track.External && !track.Forced && track.SourceIndex >= 0 && subtitleLanguageMatches(language, track.Language) && oneOf(track.Codec, "hdmv_pgs_subtitle", "dvd_subtitle") {
			tool.stream = track.SourceIndex
			break
		}
	}
	if tool.language == "" || tool.stream < 0 {
		return subtitleDraftTool{}, errors.New("no supported PGS or VobSub track matches this language")
	}
	return tool, nil
}

func selectSubtitleTranscriptionTool(config configuration.Snapshot, facts probeResult, language string) (subtitleDraftTool, error) {
	tool := subtitleDraftTool{executable: firstNonempty(config.String("binaries.whisper"), "whisper-cli"), model: config.String("subtitles.transcription_model"), architecture: firstNonempty(config.String("subtitles.transcription_architecture"), "small"), language: strings.Split(language, "-")[0], stream: -1}
	info, err := os.Stat(tool.model)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 8<<30 {
		return subtitleDraftTool{}, errors.New("configure a local Whisper model before generating a transcript")
	}
	tool.stream = subtitleTranscriptionStream(facts, language)
	if tool.stream < 0 || strings.HasSuffix(tool.architecture, ".en") && tool.language != "en" {
		return subtitleDraftTool{}, errors.New("transcription needs a matching main audio track and language-compatible model")
	}
	return tool, nil
}

// Transcription is not translation: select matching main audio, preferring its default track.
func subtitleTranscriptionStream(facts probeResult, language string) int {
	stream := -1
	for _, track := range facts.AudioFacts {
		if track.Role == "main" && track.SourceIndex >= 0 && subtitleLanguageMatches(language, track.Language) {
			stream = track.SourceIndex
			if track.Default {
				break
			}
		}
	}
	return stream
}
