package server

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *subtitleManager) fetch(request *http.Request, id, language string) (int, error) {
	canonical, validationErr := validateSubtitleLanguages([]string{language})
	if !validSubtitleItemID(id) || validationErr != nil {
		return http.StatusBadRequest, errors.New("subtitle request is invalid")
	}
	language = canonical[0]
	item, found := visibleItem(request, manager.index, id)
	if !found || item.Kind != "video" {
		return http.StatusNotFound, errors.New("not found")
	}
	if ready, _ := manager.sidecarPlanCoverage(item, language); ready {
		return http.StatusConflict, errors.New("subtitle already exists")
	}
	if err := manager.fetchSidecar(request.Context(), item, language); err != nil {
		if errors.Is(err, os.ErrExist) {
			return http.StatusConflict, errors.New("subtitle already exists")
		}
		return http.StatusBadGateway, errors.New("subtitle provider unavailable")
	}
	if err := manager.index.Refresh(request.Context()); err != nil {
		return http.StatusInternalServerError, errors.New("subtitle library could not be refreshed")
	}
	return http.StatusCreated, nil
}

func (manager *subtitleManager) restore(request *http.Request, id string) (int, error) {
	return manager.restoreLanguage(request, id, manager.settings.subtitleLanguage())
}

func (manager *subtitleManager) restoreLanguage(request *http.Request, id, language string) (int, error) {
	canonical, validationErr := validateSubtitleLanguages([]string{language})
	if validationErr != nil {
		return http.StatusBadRequest, errors.New("subtitle language is invalid")
	}
	language = canonical[0]
	item, status, err := manager.subtitleItem(request, id)
	if err != nil {
		return status, err
	}
	if err = manager.provider.restorePrevious(item, language); err != nil {
		return http.StatusConflict, err
	}
	if err = manager.index.Refresh(request.Context()); err != nil {
		return http.StatusInternalServerError, errors.New("subtitle library could not be refreshed")
	}
	return http.StatusNoContent, nil
}

func (manager *subtitleManager) setReplacement(request *http.Request, id string, replaceable bool) (int, error) {
	item, status, err := manager.subtitleItem(request, id)
	if err != nil {
		return status, err
	}
	if err = manager.provider.setReplacement(item, manager.settings.subtitleLanguage(), replaceable); err != nil {
		return http.StatusConflict, err
	}
	return http.StatusNoContent, nil
}

func (manager *subtitleManager) subtitleItem(request *http.Request, id string) (library.Item, int, error) {
	if !validSubtitleItemID(id) {
		return library.Item{}, http.StatusBadRequest, errors.New("subtitle request is invalid")
	}
	item, found := visibleItem(request, manager.index, id)
	if !found || item.Kind != "video" {
		return library.Item{}, http.StatusNotFound, errors.New("not found")
	}
	return item, http.StatusOK, nil
}

func validSubtitleItemID(id string) bool {
	if len(id) != 16 {
		return false
	}
	for _, character := range id {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func (manager *subtitleManager) fetchWantedLanguages(request *http.Request, languages []string, limit int) (int, int, error) { //nolint:cyclop,gocognit // Bounded batch work preserves provider and filesystem invariants.
	canonical, validationErr := validateSubtitleLanguages(languages)
	if validationErr != nil {
		return 0, 0, validationErr
	}
	items, err := visibleLibrary(request, manager.index)
	if err != nil {
		return 0, 0, err
	}
	attempted, written := 0, 0
	for _, item := range items {
		if item.Kind != "video" || attempted == limit {
			continue
		}
		for _, language := range canonical {
			searched, saved := manager.fetchWantedLanguage(request.Context(), item, language, attempted == limit)
			if !searched {
				continue
			}
			attempted++
			if saved {
				written++
			}
		}
	}
	if written > 0 {
		err = manager.index.Refresh(request.Context())
	}
	if attempted > written && err == nil {
		err = errors.New("one or more subtitle searches failed")
	}
	return attempted, written, err
}

func (manager *subtitleManager) fetchWantedLanguage(ctx context.Context, item library.Item, language string, atLimit bool) (bool, bool) {
	ready, _ := manager.planCoverage(ctx, item, language)
	if ready || atLimit {
		return false, false
	}
	if _, searchable := manager.searchableSubtitleLanguage(ctx, item, []string{language}); !searchable && !subtitleProviderAvailable(language) {
		return false, false
	}
	return true, manager.fetchSidecar(ctx, item, language) == nil
}

func (manager *subtitleManager) maintainLanguages(request *http.Request, languages []string, limit int) (subtitleMaintenanceResult, error) {
	items, err := visibleLibrary(request, manager.index)
	if err != nil {
		return subtitleMaintenanceResult{}, err
	}
	result := manager.maintainLanguageItems(request.Context(), items, languages, limit, 0)
	if result.Added+result.Upgraded > 0 {
		err = manager.index.Refresh(request.Context())
	}
	if result.Failed > 0 && err == nil {
		err = errors.New("one or more subtitle maintenance actions failed")
	}
	return result, err
}
