package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type subtitleEditAnchor struct {
	At     int64 `json:"atMilliseconds"`
	Offset int64 `json:"offsetMilliseconds"`
}

func (anchor *subtitleEditAnchor) UnmarshalJSON(data []byte) error {
	var value struct {
		At     *int64 `json:"atMilliseconds"`
		Offset *int64 `json:"offsetMilliseconds"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	if value.At == nil || value.Offset == nil {
		return errors.New("subtitle anchor needs both times")
	}
	anchor.At, anchor.Offset = *value.At, *value.Offset
	return nil
}

type subtitleEdit struct {
	Role          string               `json:"role,omitempty"`
	Text          string               `json:"text,omitempty"`
	DraftID       string               `json:"draftId,omitempty"`
	Language      string               `json:"language"`
	Data          []byte               `json:"data,omitempty"`
	Fingerprint   string               `json:"fingerprint"`
	Offset        int64                `json:"offsetMilliseconds"`
	Anchors       []subtitleEditAnchor `json:"anchors,omitempty"`
	AutomaticSync bool                 `json:"automaticSync"`
	Encoding      string               `json:"encoding"`
	RemoveCredits bool                 `json:"removeCredits"`
	MergeRepeated bool                 `json:"mergeRepeated"`
}

func validateSubtitleEdit(input subtitleEdit, apply bool) error {
	if !oneOf(input.Role, "", "translation", "captions") {
		return errors.New("subtitle role is invalid")
	}
	if input.DraftID != "" && (!validSubtitleFingerprint(input.DraftID) || len(input.Data) != 0) {
		return errors.New("subtitle draft is invalid")
	}
	if !validSubtitleEditBounds(input) {
		return errors.New("subtitle edit is invalid")
	}
	if input.Fingerprint != "missing" && input.Fingerprint != "" && !validSubtitleFingerprint(input.Fingerprint) || apply && input.Fingerprint == "" {
		return errors.New("preview the current subtitle before saving")
	}
	return validateSubtitleEditTiming(input)
}

func validSubtitleEditBounds(input subtitleEdit) bool {
	_, err := validateSubtitleLanguages([]string{input.Language})
	return err == nil && len(input.Data) <= 4<<20 && len(input.Text) <= 4<<20 && input.Offset >= -120000 && input.Offset <= 120000 && len(input.Anchors) <= 8 && (input.Encoding == "" || oneOf(input.Encoding, subtitleEncodings...))
}

func validateSubtitleEditTiming(input subtitleEdit) error {
	if len(input.Anchors) > 0 && input.Offset != 0 || input.AutomaticSync && (len(input.Anchors) > 0 || input.Offset != 0) {
		return errors.New("choose automatic synchronization, an offset, or timing anchors")
	}
	return validateSubtitleEditAnchors(input.Anchors)
}

func validateSubtitleEditAnchors(anchors []subtitleEditAnchor) error {
	for i, anchor := range anchors {
		if anchor.At < 0 || anchor.At > int64(36*time.Hour/time.Millisecond) || anchor.Offset < -120000 || anchor.Offset > 120000 {
			return errors.New("subtitle anchor is out of range")
		}
		if i > 0 && (anchor.At <= anchors[i-1].At || anchor.At+anchor.Offset <= anchors[i-1].At+anchors[i-1].Offset) {
			return errors.New("subtitle anchors must preserve time order")
		}
	}
	return nil
}

func validSubtitleFingerprint(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func editSubtitleTiming(document cleanedSubtitle, input subtitleEdit) (cleanedSubtitle, error) {
	if err := validateSubtitleEdit(input, false); err != nil {
		return cleanedSubtitle{}, err
	}
	if input.Offset == 0 && len(input.Anchors) == 0 {
		return document, nil
	}
	anchors := make([]subtitleAlignmentAnchor, len(input.Anchors))
	for i, anchor := range input.Anchors {
		anchors[i] = subtitleAlignmentAnchor{At: time.Duration(anchor.At) * time.Millisecond, Offset: time.Duration(anchor.Offset) * time.Millisecond}
	}
	shift := func(at time.Duration) time.Duration {
		if len(anchors) == 0 {
			return at + time.Duration(input.Offset)*time.Millisecond
		}
		return at + subtitleInterpolatedOffset(at, anchors)
	}
	cues := make([]subtitleCue, len(document.Cues))
	for i, cue := range document.Cues {
		cues[i] = subtitleCue{Start: shift(cue.Start), End: shift(cue.End), Text: cue.Text}
	}
	changed, err := subtitleDocument(cues, document.Duplicates)
	changed.Original, changed.Cleanup, changed.Synchronization = document.Original, document.Cleanup, "manual"
	changed.TimingEvidence = "manual"
	return changed, err
}

func (manager *subtitleManager) previewSubtitleEdit(request *http.Request, id string, input subtitleEdit) (subtitleReview, cleanedSubtitle, int, error) {
	if input.AutomaticSync {
		ctx, settled, err := manager.provider.admission.enter(request.Context())
		if err != nil {
			return subtitleReview{}, cleanedSubtitle{}, http.StatusConflict, err
		}
		defer settled()
		request = request.WithContext(ctx)
	}
	if err := validateSubtitleEdit(input, false); err != nil {
		return subtitleReview{}, cleanedSubtitle{}, http.StatusBadRequest, err
	}
	canonical, _ := validateSubtitleLanguages([]string{input.Language})
	input.Language = canonical[0]
	review, status, err := manager.inspectSubtitle(request, id, input.Language)
	if err != nil {
		return review, cleanedSubtitle{}, status, err
	}
	if input.Fingerprint != "" && input.Fingerprint != review.Fingerprint {
		return review, cleanedSubtitle{}, http.StatusConflict, errors.New("the subtitle changed; reload its preview")
	}
	item, _, _ := manager.subtitleItem(request, id)
	data, status, err := manager.subtitleEditSource(item, input, review.Source)
	if err != nil {
		return review, cleanedSubtitle{}, status, err
	}
	options := subtitleConversionOptions{Encoding: input.Encoding, RemoveCredits: input.RemoveCredits, MergeRepeated: input.MergeRepeated}
	options.TrackSources = subtitleEditUsesCurrentCues(input, review, data)
	document, err := convertSubtitle(data, input.Language, options)
	if err != nil {
		return review, cleanedSubtitle{}, http.StatusBadRequest, err
	}
	sources := document.SourceCues
	if input.Text != "" {
		edited, editErr := convertSubtitle([]byte(input.Text), input.Language, subtitleConversionOptions{Encoding: "utf-8", RemoveCredits: input.RemoveCredits, MergeRepeated: input.MergeRepeated})
		if editErr != nil {
			return review, cleanedSubtitle{}, http.StatusBadRequest, editErr
		}
		edited.Original = document.Original
		edited.Cleanup = append(edited.Cleanup, "Reviewed cue text")
		document = edited
	}
	document, err = manager.timeSubtitleEdit(request.Context(), item, document, input)
	if err != nil {
		return review, cleanedSubtitle{}, http.StatusConflict, errors.New("timing could not be validated; review the text and timing anchors")
	}
	view := reviewSubtitleDocument(document, firstNonempty(document.Synchronization, "Not verified against video"))
	review.Proposed = &view
	review.Comparison = subtitleReviewComparison(review.Current, &view, sources)
	review.Warnings = append(subtitleConversionWarnings(data), document.Cleanup...)
	return review, document, http.StatusOK, nil
}

func (manager *subtitleManager) writeSubtitleEdit(request *http.Request, id string, input subtitleEdit) (subtitleReview, library.Item, int, error) {
	manager.provider.sidecar.Lock()
	defer manager.provider.sidecar.Unlock()
	review, document, status, err := manager.previewSubtitleEdit(request, id, input)
	if err != nil {
		return review, library.Item{}, status, err
	}
	item, _, _ := manager.subtitleItem(request, id)
	target, err := manager.provider.openSidecar(item, review.Language)
	if err != nil {
		return review, item, http.StatusConflict, err
	}
	defer target.close()
	current, readErr := target.read("")
	exists := readErr == nil
	if !exists {
		if _, statErr := target.root.Lstat(target.name); !errors.Is(statErr, os.ErrNotExist) {
			return review, item, http.StatusConflict, errors.New("subtitle destination is unavailable")
		}
	}
	if exists && subtitleFingerprint(current) != review.Fingerprint {
		return review, item, http.StatusConflict, errors.New("the subtitle changed; reload its preview")
	}
	now := time.Now().Unix()
	record := subtitleRecord{Source: "external", CheckedAt: now, InstalledAt: now, Managed: true, Frozen: true, Cleanup: document.Cleanup, Synchronization: firstNonempty(document.Synchronization, "none"), TimingEvidence: document.TimingEvidence, Backup: exists}
	if input.DraftID != "" {
		_, record.Source, err = manager.subtitleDraftData(item, review.Language, input.DraftID)
		if err != nil {
			return review, item, http.StatusConflict, err
		}
	}
	previous, found, _ := manager.provider.ledger.record(subtitleRecordKey(id, review.Language))
	record.Role = firstNonempty(input.Role, "translation")
	reusesPrevious := input.DraftID == "" && len(input.Data) == 0 && found && previous.Fingerprint == review.Fingerprint
	if input.Role == "" && reusesPrevious {
		record.Role = previous.Role
	}
	if status, err := manager.provider.retainEditedSubtitleOriginal(document.Original, previous, &record, reusesPrevious); err != nil {
		return review, item, status, err
	}
	if status, err := manager.provider.saveSubtitleEdit(target, subtitleRecordKey(id, review.Language), current, document.Data, previous, record, exists); err != nil {
		return review, item, status, err
	}
	return review, item, http.StatusOK, nil
}

func (provider *subtitleProvider) saveSubtitleEdit(target *subtitleSidecar, key string, current, data []byte, previous, record subtitleRecord, exists bool) (int, error) {
	install := sidecarInstall{Key: key, Data: data, Record: record, Action: "added", Reason: "manual"}
	if exists {
		if err := provider.retainSubtitleRecoveryOriginal(current, previous, &install.Record); err != nil {
			return http.StatusInternalServerError, errors.New("subtitle recovery original could not be retained")
		}
		install.Current, install.Backup, install.Action = current, true, "updated"
	}
	if err := provider.installSidecar(target, install); errors.Is(err, errSidecarHistory) {
		return http.StatusInternalServerError, errSidecarHistory
	} else if err != nil {
		return http.StatusConflict, errors.New("subtitle could not be saved")
	}
	return http.StatusOK, nil
}

func (provider *subtitleProvider) retainEditedSubtitleOriginal(originalData []byte, previous subtitleRecord, record *subtitleRecord, reusesPrevious bool) (int, error) {
	if reusesPrevious && previous.OriginalFingerprint != "" {
		original, originalErr := readUpgradeSidecar(provider.originalPath(previous.OriginalFingerprint))
		if originalErr != nil || subtitleFingerprint(original) != previous.OriginalFingerprint {
			return http.StatusConflict, errors.New("retained original is unavailable; restore it before editing")
		}
		record.OriginalFingerprint = previous.OriginalFingerprint
	} else if err := provider.retainSubtitleOriginal(originalData, record); err != nil {
		return http.StatusInternalServerError, errors.New("subtitle original could not be retained")
	}
	return http.StatusOK, nil
}
