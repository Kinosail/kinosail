package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func localSubtitleTranscription(ctx context.Context, ffmpeg, whisper, media string, stream int, language, model, architecture string, duration float64) (cleanedSubtitle, []subtitleDraftWord, error) {
	directory, err := os.MkdirTemp("", "kinosail-transcription-")
	if err != nil {
		return cleanedSubtitle{}, nil, err
	}
	defer os.RemoveAll(directory)
	wav, output := filepath.Join(directory, "audio.wav"), filepath.Join(directory, "transcript")
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-nostdin", "-i", media, "-map", "0:"+strconv.Itoa(stream), "-vn", "-ac", "1", "-ar", "16000", "-t", strconv.FormatFloat(duration, 'f', 3, 64), "-fs", "1000000000", "-c:a", "pcm_s16le", wav)
	if err = command.Run(); err != nil {
		return cleanedSubtitle{}, nil, err
	}
	// DTW provides estimated token timing, not independent forced alignment.
	command = exec.CommandContext(ctx, whisper, "--model", model, "--language", language, "--file", wav, "--output-json-full", "--output-file", output, "--dtw", architecture, "--max-len", "42", "--split-on-word", "--threads", "4", "--no-prints")
	if err = command.Run(); err != nil {
		return cleanedSubtitle{}, nil, err
	}
	file, err := os.Open(output + ".json")
	if err != nil {
		return cleanedSubtitle{}, nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (32<<20)+1))
	if err != nil || len(data) > 32<<20 {
		return cleanedSubtitle{}, nil, errors.New("transcript exceeds the size limit")
	}
	return parseSubtitleTranscript(data, language, duration)
}

type subtitleTranscriptTimes struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

func parseSubtitleTranscript(data []byte, language string, duration float64) (cleanedSubtitle, []subtitleDraftWord, error) {
	var output struct {
		Result struct {
			Language string `json:"language"`
		} `json:"result"`
		Transcription []subtitleTranscriptSegment `json:"transcription"`
	}
	invalid := errors.New("local transcript is invalid")
	if math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= 0 || duration > subtitleAudioLimit.Seconds() || len(data) == 0 || len(data) > 32<<20 || !utf8.Valid(data) || json.Unmarshal(data, &output) != nil || output.Result.Language != language || len(output.Transcription) == 0 || len(output.Transcription) > 20000 {
		return cleanedSubtitle{}, nil, invalid
	}
	cues := make([]subtitleCue, 0, len(output.Transcription))
	words := []subtitleDraftWord{}
	previous := int64(-1)
	for _, segment := range output.Transcription {
		if !segment.valid(previous, duration) {
			return cleanedSubtitle{}, nil, invalid
		}
		text := strings.TrimSpace(segment.Text)
		if text == "" {
			continue
		}
		previous = segment.Offsets.From
		cues = append(cues, subtitleCue{Start: time.Duration(segment.Offsets.From) * time.Millisecond, End: time.Duration(segment.Offsets.To) * time.Millisecond, Text: text})
		firstWord := len(words)
		var err error
		words, err = transcriptWords(segment, words, firstWord, language, duration)
		if err != nil {
			return cleanedSubtitle{}, nil, err
		}
	}
	document, err := subtitleDocument(cues, 0)
	if err == nil {
		document.Original = append([]byte(nil), document.Data...)
		document.TimingEvidence = "unverified"
	}
	return document, words, err
}

type subtitleTranscriptToken struct {
	Text        string                   `json:"text"`
	Probability float64                  `json:"p"`
	DTW         float64                  `json:"t_dtw"`
	Offsets     *subtitleTranscriptTimes `json:"offsets"`
}

type subtitleTranscriptSegment struct {
	Offsets subtitleTranscriptTimes   `json:"offsets"`
	Text    string                    `json:"text"`
	Tokens  []subtitleTranscriptToken `json:"tokens"`
}

func transcriptWords(segment subtitleTranscriptSegment, words []subtitleDraftWord, firstWord int, language string, duration float64) ([]subtitleDraftWord, error) {
	invalid := errors.New("local transcript is invalid")
	for _, token := range segment.Tokens {
		if !token.valid() {
			return nil, invalid
		}
		if strings.HasPrefix(token.Text, "[_") || strings.TrimSpace(token.Text) == "" {
			continue
		}
		start, end := token.timing(segment.Offsets, duration)
		text := strings.TrimSpace(token.Text)
		boundary := len(words) == firstWord || token.wordBoundary(language)
		if boundary {
			words = append(words, subtitleDraftWord{Text: text, Start: start, End: end, Confidence: token.Probability})
		} else {
			word := &words[len(words)-1]
			word.Text += text
			word.Start, word.End, word.Confidence = min(word.Start, start), max(word.End, end), min(word.Confidence, token.Probability)
			if len(word.Text) > 4096 {
				return nil, invalid
			}
		}
		if len(words) > 100000 {
			return nil, invalid
		}
	}
	return words, nil
}

func (segment subtitleTranscriptSegment) valid(previous int64, duration float64) bool {
	return !(segment.Offsets.From < 0 || segment.Offsets.From < previous || segment.Offsets.To <= segment.Offsets.From || float64(segment.Offsets.To)/1000 > duration+2 || !validSubtitleDraftText(segment.Text) || len(segment.Tokens) > 2048)
}

func (token subtitleTranscriptToken) valid() bool {
	return !(!validSubtitleDraftText(token.Text) || math.IsNaN(token.Probability) || math.IsInf(token.Probability, 0) || token.Probability < 0 || token.Probability > 1 || math.IsNaN(token.DTW) || math.IsInf(token.DTW, 0))
}

func (token subtitleTranscriptToken) timing(offsets subtitleTranscriptTimes, duration float64) (float64, float64) {
	start, end := float64(offsets.From)/1000, float64(offsets.To)/1000
	if token.Offsets != nil && token.Offsets.From >= 0 && token.Offsets.To >= token.Offsets.From && float64(token.Offsets.To)/1000 <= duration+2 {
		start, end = float64(token.Offsets.From)/1000, float64(token.Offsets.To)/1000
	}
	// The CLI's DTW timestamp uses 10 ms ticks. Preserve it as a word/token
	// inspection anchor; it does not change the segment's validated bounds.
	if token.DTW >= 0 && token.DTW/100 >= start && token.DTW/100 <= end {
		start = token.DTW / 100
	}
	return start, end
}

func (token subtitleTranscriptToken) wordBoundary(language string) bool {
	return strings.TrimLeftFunc(token.Text, unicode.IsSpace) != token.Text || language == "zh" || language == "ja"
}
