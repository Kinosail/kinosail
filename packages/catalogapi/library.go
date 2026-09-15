package catalogapi

import (
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

// Library returns Player's canonical Library browse endpoint.
func Library(browse func(*http.Request) (catalog.Result, error), project func(*http.Request, library.Item) any) http.HandlerFunc {
	if browse == nil || project == nil {
		return func(writer http.ResponseWriter, _ *http.Request) {
			apiAction{err: errors.New("Library API is unavailable"), status: http.StatusInternalServerError}.serve(writer)
		}
	}
	return func(writer http.ResponseWriter, request *http.Request) {
		page, err := browse(request)
		if err != nil {
			status := http.StatusServiceUnavailable
			if errors.Is(err, catalog.ErrInvalidBrowse) {
				status = http.StatusBadRequest
			}
			apiAction{err: err, status: status}.serve(writer)
			return
		}
		items := make([]any, 0, len(page.Items))
		for _, item := range page.Items {
			items = append(items, project(request, item))
		}
		apiAction{body: map[string]any{
			"items": items, "view": page.View, "sort": page.Sort, "query": page.Query,
			"letter": page.Letter, "letters": page.Letters, "total": page.Total,
			"offset": page.Offset, "limit": page.Limit,
		}, status: http.StatusOK}.serve(writer)
	}
}
