package server

import (
	"context"
	"net/http"

	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *subtitleManager) maintainItems(ctx context.Context, items []library.Item, language string, limit, start int) subtitleMaintenanceResult {
	return manager.maintainLanguageItems(ctx, items, []string{language}, limit, start)
}

func (manager *subtitleManager) fetchWanted(request *http.Request, language string, limit int) (int, int, error) {
	return manager.fetchWantedLanguages(request, []string{language}, limit)
}

func (manager *subtitleManager) maintain(request *http.Request, language string, limit int) (subtitleMaintenanceResult, error) {
	return manager.maintainLanguages(request, []string{language}, limit)
}
