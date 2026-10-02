package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
)

func TestCatalogAPIStopsCancelledMetadataSearch(t *testing.T) {
	handler, baseRequest := cancelledMetadataSearchFixture()
	for _, condition := range []string{"cancelled", "expired", "expires-during-search"} {
		t.Run(condition, func(t *testing.T) {
			request := baseRequest
			var ctx context.Context
			var cancel context.CancelFunc
			switch condition {
			case "cancelled":
				ctx, cancel = context.WithCancel(request.Context())
				cancel()
			case "expired":
				ctx, cancel = context.WithDeadline(request.Context(), time.Now().Add(-time.Second))
			default:
				ctx, cancel = context.WithTimeout(request.Context(), time.Millisecond)
			}
			defer cancel()
			request = request.WithContext(ctx)
			response := httptest.NewRecorder()
			handler(response, request)
			if ctx.Err() == nil || response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), `"items"`) {
				t.Fatalf("cancelled browse status=%d body_bytes=%d context_error=%v", response.Code, response.Body.Len(), ctx.Err())
			}
		})
	}
}

func cancelledMetadataSearchFixture() (http.HandlerFunc, *http.Request) {
	items := benchmarkLibraryItems(10000)
	for position := range items {
		items[position].Plot = strings.Repeat("A quiet journey. ", 120) + "Café"
	}
	index := memoryLibraryIndex(items, true)
	progress, lists := newProgressStore(""), newListStore("")
	handler := catalogapi.Library(func(request *http.Request) (catalog.Result, error) {
		return browseLibrary(request, index, progress, lists)
	}, progress.ClientItem)
	return handler, ownerRequest("/api/v1/library?view=movies&q=Movie+9999")
}
