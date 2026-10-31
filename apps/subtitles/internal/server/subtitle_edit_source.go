package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *subtitleManager) subtitleEditSource(item library.Item, input subtitleEdit, source string) ([]byte, int, error) {
	data := input.Data
	if input.DraftID != "" {
		var err error
		data, _, err = manager.subtitleDraftData(item, input.Language, input.DraftID)
		if err != nil {
			return nil, http.StatusConflict, err
		}
	}
	if len(data) > 0 {
		return data, http.StatusOK, nil
	}
	if source == "subsource" {
		return nil, http.StatusConflict, errors.New("this provider requires its subtitle to remain unchanged")
	}
	_, data, err := manager.provider.reviewSource(item, input.Language)
	if err != nil {
		return nil, http.StatusNotFound, errors.New("choose a subtitle file to import")
	}
	return data, http.StatusOK, nil
}

func (manager *subtitleManager) timeSubtitleEdit(ctx context.Context, item library.Item, document cleanedSubtitle, input subtitleEdit) (cleanedSubtitle, error) {
	document, err := editSubtitleTiming(document, input)
	if err != nil || !input.AutomaticSync {
		return document, err
	}
	probabilities, err := manager.provider.sync.speechForLanguage(ctx, item, input.Language)
	if err != nil {
		return document, err
	}
	document, _, err = synchronizeSubtitle(probabilities, document)
	return document, err
}
