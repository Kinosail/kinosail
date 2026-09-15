package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/MikeO7/kinosail/packages/library"
)

// Drafts are intentionally separate from maintenance: OCR and recognition can
// invent or miss dialogue. Only a reviewed, fingerprint-checked edit installs one.
type subtitleDraft struct {
	ID       string                  `json:"id"`
	Item     string                  `json:"item"`
	Language string                  `json:"language"`
	Method   string                  `json:"method"`
	State    string                  `json:"state"`
	Message  string                  `json:"message"`
	Words    []subtitleDraftWord     `json:"words,omitempty"`
	Document *subtitleReviewDocument `json:"document,omitempty"`
	data     []byte
	version  string
	cancel   context.CancelFunc
}

type subtitleDraftWord struct {
	Text       string  `json:"text"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Confidence float64 `json:"confidence"`
}

type subtitleDraftStore struct {
	sync.Mutex
	closed  bool
	current subtitleDraft
}

type subtitleDraftInput struct {
	Language string `json:"language"`
	Method   string `json:"method"`
	Action   string `json:"action"`
	DraftID  string `json:"draftId"`
}

func validateSubtitleDraft(input subtitleDraftInput) error {
	if _, err := validateSubtitleLanguages([]string{input.Language}); err != nil || !oneOf(input.Action, "start", "cancel") || !oneOf(input.Method, "ocr", "transcription") || input.Action == "start" && input.DraftID != "" || input.Action == "cancel" && !validSubtitleFingerprint(input.DraftID) {
		return errors.New("local subtitle draft request is invalid")
	}
	return nil
}

func subtitleMediaVersion(item library.Item) (string, error) {
	info, err := os.Stat(item.Path)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("video is unavailable")
	}
	return subtitleFingerprint([]byte(item.Path + "\x00" + strconv.FormatInt(info.Size(), 10) + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10))), nil
}

func (manager *subtitleManager) localSubtitleDraft(request *http.Request, id string, input subtitleDraftInput) (subtitleDraft, int, error) {
	if err := validateSubtitleDraft(input); err != nil {
		return subtitleDraft{}, http.StatusBadRequest, err
	}
	languages, _ := validateSubtitleLanguages([]string{input.Language})
	input.Language = languages[0]
	item, status, err := manager.subtitleItem(request, id)
	if err != nil {
		return subtitleDraft{}, status, err
	}
	manager.drafts.Lock()
	defer manager.drafts.Unlock()
	current := &manager.drafts.current
	if manager.drafts.closed {
		return subtitleDraft{}, http.StatusServiceUnavailable, errors.New("server is stopping")
	}
	if input.Action == "cancel" {
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
	if current.cancel != nil {
		return subtitleDraft{}, http.StatusConflict, errors.New("another local draft is running; wait or cancel it first")
	}
	if manager.probe == nil || manager.probe.ffmpeg == "" {
		return subtitleDraft{}, http.StatusConflict, errors.New("local FFmpeg and FFprobe are required")
	}
	facts := manager.probe.facts(request.Context(), item)
	if math.IsNaN(facts.Duration) || math.IsInf(facts.Duration, 0) || facts.Duration <= 0 || facts.Duration > subtitleAudioLimit.Seconds() {
		return subtitleDraft{}, http.StatusConflict, errors.New("local drafts require a video with a known duration up to eight hours")
	}
	config := manager.settings.configuration()
	executable, model, architecture := config.String("binaries.tesseract"), "", ""
	stream, language := -1, ""
	if input.Method == "ocr" {
		executable = firstNonempty(executable, "tesseract")
		language = subtitleOCRLanguage(input.Language)
		for _, track := range facts.SubtitleFacts {
			if !track.Text && !track.External && !track.Forced && track.SourceIndex >= 0 && subtitleLanguageMatches(input.Language, track.Language) && oneOf(track.Codec, "hdmv_pgs_subtitle", "dvd_subtitle") {
				stream = track.SourceIndex
				break
			}
		}
		if language == "" || stream < 0 {
			return subtitleDraft{}, http.StatusConflict, errors.New("no supported PGS or VobSub track matches this language")
		}
	} else {
		executable = firstNonempty(config.String("binaries.whisper"), "whisper-cli")
		model, architecture = config.String("subtitles.transcription_model"), firstNonempty(config.String("subtitles.transcription_architecture"), "small")
		info, modelErr := os.Stat(model)
		if modelErr != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 8<<30 {
			return subtitleDraft{}, http.StatusConflict, errors.New("configure a local Whisper model before generating a transcript")
		}
		// Transcription is not translation: never label a different spoken language as the requested language.
		for _, track := range facts.AudioFacts {
			if track.Role == "main" && track.SourceIndex >= 0 && subtitleLanguageMatches(input.Language, track.Language) {
				stream = track.SourceIndex
				if track.Default {
					break
				}
			}
		}
		language = strings.Split(input.Language, "-")[0]
		if stream < 0 || strings.HasSuffix(architecture, ".en") && language != "en" {
			return subtitleDraft{}, http.StatusConflict, errors.New("transcription needs a matching main audio track and language-compatible model")
		}
	}
	if _, err = exec.LookPath(executable); err != nil {
		return subtitleDraft{}, http.StatusConflict, errors.New("the local recognition tool is unavailable; check configuration")
	}
	if _, err = exec.LookPath(manager.probe.ffmpeg); err != nil {
		return subtitleDraft{}, http.StatusConflict, errors.New("local FFmpeg is unavailable")
	}
	version, err := subtitleMediaVersion(item)
	if err != nil {
		return subtitleDraft{}, http.StatusConflict, err
	}
	token := make([]byte, 32)
	if _, err = rand.Read(token); err != nil {
		return subtitleDraft{}, http.StatusInternalServerError, errors.New("local draft could not be started")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	*current = subtitleDraft{ID: hex.EncodeToString(token), Item: id, Language: input.Language, Method: input.Method, State: "running", Message: "Generating locally. Review every result before saving.", version: version, cancel: cancel}
	draft := *current
	go manager.generateSubtitleDraft(ctx, item, draft, stream, language, executable, model, architecture, facts.Duration)
	return draft, http.StatusAccepted, nil
}

func (manager *subtitleManager) generateSubtitleDraft(ctx context.Context, item library.Item, draft subtitleDraft, stream int, language, executable, model, architecture string, duration float64) {
	defer draft.cancel()
	var document cleanedSubtitle
	var words []subtitleDraftWord
	var err error
	if draft.Method == "ocr" {
		document, words, err = localSubtitleOCR(ctx, manager.probe.ffmpeg, executable, item.Path, stream, language, duration)
	} else {
		document, words, err = localSubtitleTranscription(ctx, manager.probe.ffmpeg, executable, item.Path, stream, language, model, architecture, duration)
	}
	version, versionErr := subtitleMediaVersion(item)
	manager.drafts.Lock()
	defer manager.drafts.Unlock()
	current := &manager.drafts.current
	if current.ID != draft.ID {
		return
	}
	current.cancel = nil
	if ctx.Err() != nil {
		current.State, current.Message = "stopped", "Local generation stopped or reached its two-hour limit."
		return
	}
	if err != nil || versionErr != nil || version != draft.version {
		current.State, current.Message = "failed", "Local generation could not produce a complete draft. Check the source track, installed language data, and model configuration."
		return
	}
	view := reviewSubtitleDocument(document, "Generated timing; review against dialogue")
	current.State, current.Message, current.Document, current.Words, current.data = "ready", "Draft ready. Recognition can miss or invent words; compare with the video before saving.", &view, words, document.Data
}

func (manager *subtitleManager) subtitleDraftData(item library.Item, language, draftID string) ([]byte, string, error) {
	manager.drafts.Lock()
	defer manager.drafts.Unlock()
	draft := manager.drafts.current
	version, err := subtitleMediaVersion(item)
	if err != nil || draft.ID != draftID || draft.Item != item.ID || draft.Language != language || draft.State != "ready" || version != draft.version {
		return nil, "", errors.New("local draft is unavailable or its video changed; generate it again")
	}
	return append([]byte(nil), draft.data...), draft.Method, nil
}

func validSubtitleDraftText(text string) bool {
	return len(text) <= 4096 && utf8.ValidString(text) && strings.IndexFunc(text, func(character rune) bool {
		return unicode.IsControl(character) && character != '\n' && character != '\t'
	}) < 0
}
