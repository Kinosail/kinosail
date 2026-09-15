package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"
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
	if _, err := validateSubtitleLanguages([]string{input.Language}); err != nil || (len(input.Data) > 4<<20 || len(input.Text) > 4<<20) || input.Offset < -120000 || input.Offset > 120000 || len(input.Anchors) > 8 || input.Encoding != "" && !oneOf(input.Encoding, subtitleEncodings...) {
		return errors.New("subtitle edit is invalid")
	}
	if input.Fingerprint != "missing" && input.Fingerprint != "" && !validSubtitleFingerprint(input.Fingerprint) || apply && input.Fingerprint == "" {
		return errors.New("preview the current subtitle before saving")
	}
	if len(input.Anchors) > 0 && input.Offset != 0 || input.AutomaticSync && (len(input.Anchors) > 0 || input.Offset != 0) {
		return errors.New("choose automatic synchronization, an offset, or timing anchors")
	}
	for i, anchor := range input.Anchors {
		if anchor.At < 0 || anchor.At > int64(36*time.Hour/time.Millisecond) || anchor.Offset < -120000 || anchor.Offset > 120000 {
			return errors.New("subtitle anchor is out of range")
		}
		if i > 0 && (anchor.At <= input.Anchors[i-1].At || anchor.At+anchor.Offset <= input.Anchors[i-1].At+input.Anchors[i-1].Offset) {
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
	data := input.Data
	if input.DraftID != "" {
		data, _, err = manager.subtitleDraftData(item, input.Language, input.DraftID)
		if err != nil {
			return review, cleanedSubtitle{}, http.StatusConflict, err
		}
	}
	if len(data) == 0 {
		if review.Source == "subsource" {
			return review, cleanedSubtitle{}, http.StatusConflict, errors.New("this provider requires its subtitle to remain unchanged")
		}
		_, data, err = subtitleReviewSource(item, input.Language)
		if err != nil {
			return review, cleanedSubtitle{}, http.StatusNotFound, errors.New("choose a subtitle file to import")
		}
	}
	options := subtitleConversionOptions{Encoding: input.Encoding, RemoveCredits: input.RemoveCredits, MergeRepeated: input.MergeRepeated}
	document, err := convertSubtitle(data, input.Language, options)
	if err != nil {
		return review, cleanedSubtitle{}, http.StatusBadRequest, err
	}
	if input.Text != "" {
		edited, editErr := convertSubtitle([]byte(input.Text), input.Language, subtitleConversionOptions{Encoding: "utf-8", RemoveCredits: input.RemoveCredits, MergeRepeated: input.MergeRepeated})
		if editErr != nil {
			return review, cleanedSubtitle{}, http.StatusBadRequest, editErr
		}
		edited.Original = document.Original
		edited.Cleanup = append(edited.Cleanup, "Reviewed cue text")
		document = edited
	}
	document, err = editSubtitleTiming(document, input)
	if err == nil && input.AutomaticSync {
		var probabilities []float64
		probabilities, err = manager.provider.sync.speechForLanguage(request.Context(), item, input.Language)
		if err == nil {
			document, _, err = synchronizeSubtitle(probabilities, document)
		}
	}
	if err != nil {
		return review, cleanedSubtitle{}, http.StatusConflict, errors.New("timing could not be validated; review the text and timing anchors")
	}
	view := reviewSubtitleDocument(document, firstNonempty(document.Synchronization, "Not verified against video"))
	review.Proposed = &view
	review.Warnings = append(subtitleConversionWarnings(data), document.Cleanup...)
	return review, document, http.StatusOK, nil
}

func (manager *subtitleManager) applySubtitleEdit(request *http.Request, id string, input subtitleEdit) (subtitleReview, int, error) {
	if err := validateSubtitleEdit(input, true); err != nil {
		return subtitleReview{}, http.StatusBadRequest, err
	}
	// Serialize edits with automatic upgrades and re-read under the lock.
	manager.provider.sidecar.Lock()
	defer manager.provider.sidecar.Unlock()
	review, document, status, err := manager.previewSubtitleEdit(request, id, input)
	if err != nil {
		return review, status, err
	}
	item, _, _ := manager.subtitleItem(request, id)
	target := subtitleSidecarPath(item, review.Language)
	current, readErr := readUpgradeSidecar(target)
	exists := readErr == nil
	if !exists {
		if _, statErr := os.Lstat(target); !errors.Is(statErr, os.ErrNotExist) {
			return review, http.StatusConflict, errors.New("subtitle destination is unavailable")
		}
	}
	if exists && subtitleFingerprint(current) != review.Fingerprint {
		return review, http.StatusConflict, errors.New("the subtitle changed; reload its preview")
	}
	now := time.Now().Unix()
	record := subtitleRecord{Source: "external", CheckedAt: now, InstalledAt: now, Managed: true, Frozen: true, Cleanup: document.Cleanup, Synchronization: firstNonempty(document.Synchronization, "none"), TimingEvidence: document.TimingEvidence, Backup: exists}
	if input.DraftID != "" {
		_, record.Source, err = manager.subtitleDraftData(item, review.Language, input.DraftID)
		if err != nil {
			return review, http.StatusConflict, err
		}
	}
	previous, found, _ := manager.provider.ledger.record(subtitleRecordKey(id, review.Language))
	record.Role = firstNonempty(input.Role, "translation")
	if input.Role == "" && input.DraftID == "" && len(input.Data) == 0 && found && previous.Fingerprint == review.Fingerprint {
		record.Role = previous.Role
	}
	if len(input.Data) == 0 && input.DraftID == "" && found && previous.Fingerprint == review.Fingerprint && previous.OriginalFingerprint != "" {
		original, originalErr := readUpgradeSidecar(manager.provider.originalPath(previous.OriginalFingerprint))
		if originalErr != nil || subtitleFingerprint(original) != previous.OriginalFingerprint {
			return review, http.StatusConflict, errors.New("retained original is unavailable; restore it before editing")
		}
		record.OriginalFingerprint = previous.OriginalFingerprint
	} else if err = manager.provider.retainSubtitleOriginal(document.Original, &record); err != nil {
		return review, http.StatusInternalServerError, errors.New("subtitle original could not be retained")
	}
	if exists {
		if err = manager.provider.retainSubtitleRecoveryOriginal(current, previous, &record); err != nil {
			return review, http.StatusInternalServerError, errors.New("subtitle recovery original could not be retained")
		}
		if err = saveSubtitle(target+".kinosail.bak", current); err == nil {
			err = saveSubtitle(target, document.Data)
		}
	} else {
		err = saveSubtitleExclusive(target, document.Data)
	}
	if err != nil {
		return review, http.StatusConflict, errors.New("subtitle could not be saved")
	}
	if err = manager.provider.ledger.store(subtitleRecordKey(id, review.Language), completeSubtitleRecord(target, document.Data, record)); err != nil {
		if exists {
			_ = saveSubtitle(target, current)
		} else {
			_ = os.Remove(target)
		}
		return review, http.StatusInternalServerError, errors.New("subtitle history could not be saved")
	}
	if err = manager.index.Refresh(request.Context()); err != nil {
		return review, http.StatusInternalServerError, errors.New("subtitle was saved but the library could not be refreshed")
	}
	return manager.inspectSubtitle(request, id, review.Language)
}
