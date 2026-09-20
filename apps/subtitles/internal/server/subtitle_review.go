package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/library"
)

type subtitleReviewCue struct {
	Start    float64  `json:"start"`
	End      float64  `json:"end"`
	Text     string   `json:"text"`
	Warnings []string `json:"warnings,omitempty"`
}

type subtitleQuality struct {
	CueCount     int     `json:"cueCount"`
	FastCues     int     `json:"fastCues"`
	Overlaps     int     `json:"overlaps"`
	LongLines    int     `json:"longLines"`
	MaxCPS       float64 `json:"maxCPS"`
	FirstCue     float64 `json:"firstCue"`
	LastCue      float64 `json:"lastCue"`
	Timing       string  `json:"timing"`
	Completeness string  `json:"completeness"`
}

type subtitleReviewDocument struct {
	Cues    []subtitleReviewCue `json:"cues"`
	Quality subtitleQuality     `json:"quality"`
}

type subtitleReview struct {
	ID                string                  `json:"id"`
	Title             string                  `json:"title"`
	Language          string                  `json:"language"`
	Fingerprint       string                  `json:"fingerprint"`
	MatchEvidence     string                  `json:"matchEvidence"`
	Role              string                  `json:"role"`
	Source            string                  `json:"source"`
	OriginalAvailable bool                    `json:"originalAvailable"`
	Restorable        bool                    `json:"restorable"`
	Current           *subtitleReviewDocument `json:"current,omitempty"`
	Proposed          *subtitleReviewDocument `json:"proposed,omitempty"`
	Warnings          []string                `json:"warnings"`
	Duration          float64                 `json:"duration"`
}

func reviewSubtitleDocument(document cleanedSubtitle, timing string) subtitleReviewDocument {
	result := subtitleReviewDocument{Cues: make([]subtitleReviewCue, 0, len(document.Cues)), Quality: subtitleQuality{CueCount: len(document.Cues), MaxCPS: document.MaxCPS, Timing: timing, Completeness: "Needs a language and dialogue review; timing cannot prove completeness"}}
	var previousEnd time.Duration
	for _, cue := range document.Cues {
		view := subtitleReviewCue{Start: cue.Start.Seconds(), End: cue.End.Seconds(), Text: cue.Text}
		plain := subtitleTags.ReplaceAllString(cue.Text, "")
		if float64(utf8.RuneCountInString(plain))/(cue.End-cue.Start).Seconds() > 20 {
			result.Quality.FastCues++
			view.Warnings = append(view.Warnings, "Fast reading speed")
		}
		if cue.Start < previousEnd {
			result.Quality.Overlaps++
			view.Warnings = append(view.Warnings, "Overlapping dialogue")
		}
		for _, line := range strings.Split(plain, "\n") {
			if utf8.RuneCountInString(line) > 42 {
				result.Quality.LongLines++
				view.Warnings = append(view.Warnings, "Long line")
				break
			}
		}
		previousEnd = max(previousEnd, cue.End)
		result.Cues = append(result.Cues, view)
	}
	if len(document.Cues) > 0 {
		result.Quality.FirstCue = document.Cues[0].Start.Seconds()
		result.Quality.LastCue = previousEnd.Seconds()
	}
	return result
}

func (manager *subtitleManager) inspectSubtitle(request *http.Request, id, language string) (subtitleReview, int, error) {
	languages, err := validateSubtitleLanguages([]string{language})
	if err != nil {
		return subtitleReview{}, http.StatusBadRequest, errors.New("subtitle language is invalid")
	}
	language = languages[0]
	item, status, err := manager.subtitleItem(request, id)
	if err != nil {
		return subtitleReview{}, status, err
	}
	result := subtitleReview{ID: id, Title: item.Title, Language: language, Fingerprint: "missing", Source: "Not installed", MatchEvidence: "No verified identity evidence", Role: "translation", Warnings: []string{}}
	sourcePath, data, err := subtitleReviewSource(item, language)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return subtitleReview{}, http.StatusConflict, errors.New("subtitle file is unavailable")
	}
	if len(data) > 0 {
		result.Fingerprint = subtitleFingerprint(data)
		result.Source = "Local file"
		result.Role = subtitleRoleFromPath(sourcePath)
		document, conversionErr := convertSubtitle(data, language, subtitleConversionOptions{})
		if conversionErr != nil {
			result.Warnings = append(result.Warnings, "The installed file needs an encoding override or a supported text format")
		} else {
			view := reviewSubtitleDocument(document, "Not verified against video")
			result.Current = &view
			result.Warnings = append(result.Warnings, subtitleConversionWarnings(data)...)
		}
	}
	record, found, err := manager.provider.ledger.record(subtitleRecordKey(id, language))
	if err != nil {
		return subtitleReview{}, http.StatusServiceUnavailable, errors.New("subtitle history is unavailable")
	}
	if found && record.Fingerprint == result.Fingerprint {
		result.Source, result.Restorable = record.Source, record.Backup
		result.Role = firstNonempty(record.Role, result.Role)
		switch {
		case record.TimingEvidence == "file-hash":
			result.MatchEvidence = "Provider matched the exact video file hash"
		case record.Source == "embedded":
			result.MatchEvidence = "Text extracted from this video"
		case record.Source == "ocr" || record.Source == "transcription":
			result.MatchEvidence = "Locally generated and reviewed"
		case record.ReleaseMatch >= .8:
			result.MatchEvidence = "Strong release filename compatibility; timing is separate"
		case record.ReleaseMatch > 0:
			result.MatchEvidence = "Title matched; release timing required audio validation"
		default:
			result.MatchEvidence = "Owner-supplied subtitle; identity needs review"
		}
		result.OriginalAvailable = record.OriginalFingerprint != ""
		if result.Current != nil {
			result.Current.Quality.Timing = subtitleTimingEvidence(record.TimingEvidence, record.Synchronization)
		}
	}
	if manager.probe != nil {
		result.Duration = manager.probe.facts(request.Context(), item).Duration
	}
	return result, http.StatusOK, nil
}

func subtitleReviewSource(item library.Item, language string) (string, []byte, error) {
	if !filepath.IsLocal(language) || filepath.Base(language) != language {
		return "", nil, errors.New("subtitle language is invalid")
	}
	canonical, err := validateSubtitleLanguages([]string{language})
	if err != nil {
		return "", nil, err
	}
	language = canonical[0]
	target := subtitleSidecarPath(item, language)
	if _, err := os.Lstat(target); err == nil {
		data, readErr := readUpgradeSidecar(target)
		return target, data, readErr
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", nil, err
	}
	for _, path := range item.Subtitles {
		if subtitleLanguageFromPath(item.Path, path) == language && subtitleRoleFromPath(path) != "forced" {
			data, err := readUpgradeSidecar(path)
			return path, data, err
		}
	}
	return target, nil, os.ErrNotExist
}

func subtitleConversionWarnings(data []byte) []string {
	warnings := []string{}
	text := string(data)
	if !utf8.Valid(data) {
		warnings = append(warnings, "Legacy encoding: verify names and non-Western characters in the preview")
	}
	if strings.Contains(text, "[Events]") || strings.Contains(text, "<tt") {
		warnings = append(warnings, "SRT keeps dialogue and cue timing; advanced styles, regions, animation, and karaoke need the original file")
	}
	if strings.Contains(text, "WEBVTT") {
		warnings = append(warnings, "SRT cannot retain WebVTT cue positioning, regions, or stylesheet rules")
	}
	return warnings
}

func (manager *subtitleManager) exportSubtitle(request *http.Request, id, language, format string) ([]byte, string, int, error) {
	if !oneOf(format, "srt", "vtt", "original") || !validLanguage(language) {
		return nil, "", http.StatusBadRequest, errors.New("subtitle export is invalid")
	}
	item, status, err := manager.subtitleItem(request, id)
	if err != nil {
		return nil, "", status, err
	}
	_, data, err := subtitleReviewSource(item, language)
	if err != nil {
		return nil, "", http.StatusNotFound, errors.New("subtitle is unavailable")
	}
	if format == "original" {
		record, found, ledgerErr := manager.provider.ledger.record(subtitleRecordKey(id, language))
		if ledgerErr != nil || !found || record.Fingerprint != subtitleFingerprint(data) || record.OriginalFingerprint == "" {
			return nil, "", http.StatusNotFound, errors.New("subtitle original is unavailable")
		}
		data, err = readUpgradeSidecar(manager.provider.originalPath(record.OriginalFingerprint))
		if err != nil || subtitleFingerprint(data) != record.OriginalFingerprint {
			return nil, "", http.StatusConflict, errors.New("subtitle original could not be verified")
		}
		return data, "application/octet-stream", http.StatusOK, nil
	}
	// Exporting the same format keeps its positions and styling intact.
	if format == "vtt" && strings.HasPrefix(strings.TrimSpace(string(data)), "WEBVTT") {
		return data, "text/vtt; charset=utf-8", http.StatusOK, nil
	}
	document, err := convertSubtitle(data, language, subtitleConversionOptions{})
	if err != nil {
		return nil, "", http.StatusConflict, err
	}
	if format == "srt" {
		return document.Data, "application/x-subrip; charset=utf-8", http.StatusOK, nil
	}
	var output strings.Builder
	output.WriteString("WEBVTT\n\n")
	for _, cue := range document.Cues {
		output.WriteString(strings.ReplaceAll(formatSubtitleTime(cue.Start), ",", ".") + " --> " + strings.ReplaceAll(formatSubtitleTime(cue.End), ",", ".") + "\n" + cue.Text + "\n\n")
	}
	return []byte(output.String()), "text/vtt; charset=utf-8", http.StatusOK, nil
}

func subtitleOriginalName(format string, data []byte) string {
	if format == "original" {
		format = "txt"
		decoded, err := decodeSubtitleText(data)
		if err == nil {
			text := strings.TrimSpace(string(decoded))
			switch {
			case strings.HasPrefix(text, "WEBVTT"):
				format = "vtt"
			case strings.HasPrefix(text, "[Script Info]"):
				format = "ass"
			case strings.HasPrefix(text, "<?xml"), strings.HasPrefix(text, "<tt"):
				format = "ttml"
			case strings.Contains(text, "-->"):
				format = "srt"
			}
		}
	}
	return filepath.Base("subtitle." + format)
}

func subtitleTimingEvidence(evidence, correction string) string {
	switch evidence {
	case "audio":
		return "Checked against dialogue across the video · " + correction
	case "file-hash":
		return "Exact file hash match; audio not analyzed"
	case "embedded":
		return "Timing from the embedded text track"
	case "manual":
		return "Manually reviewed timing"
	default:
		return "Not verified against video"
	}
}
