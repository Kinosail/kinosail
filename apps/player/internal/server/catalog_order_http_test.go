package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestCatalogHTTPPublicationRejectsLateOldOrder(t *testing.T) { //nolint:cyclop,funlen // One forced interleaving checks old-response stability and new-response freshness.
	index := catalog.NewMemoryIndex([]library.Item{{ID: "a", Title: "Alpha"}, {ID: "b", Title: "Beta"}}, true)
	var pm sync.Mutex
	var lm sync.RWMutex
	progress, listed := map[string]catalog.PlaybackState{}, map[string]bool{}
	captured, resume := make(chan struct{}), make(chan struct{})
	var entered atomic.Bool
	var release sync.Once
	defer release.Do(func() { close(resume) })
	handler := catalogapi.Library(func(request *http.Request) (catalog.Result, error) {
		return index.BrowseLibrary(request.Context(), request.URL.Query(), "en", func() catalog.BrowseAccess {
			if !entered.Swap(true) {
				close(captured)
				<-resume
			}
			return catalog.NewBrowseAccess(&pm, &lm, &progress, &listed, "owner", true, func(library.Item) bool { return true })
		})
	}, func(_ *http.Request, item library.Item) any {
		return map[string]string{"id": item.ID, "title": item.Title}
	})
	first := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler(first, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
		close(done)
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	select {
	case <-captured:
	case <-ctx.Done():
		t.Fatal("request did not capture its snapshot")
	}
	index.SetDecorator(func(items []library.Item) []library.Item {
		items[0].Title, items[1].Title = "Zebra", "Apple"
		return items
	})
	release.Do(func() { close(resume) })
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("old request did not complete")
	}
	second := httptest.NewRecorder()
	handler(second, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	for _, check := range []struct {
		response    *httptest.ResponseRecorder
		ids, titles []string
	}{
		{first, []string{"a", "b"}, []string{"Alpha", "Beta"}},
		{second, []string{"b", "a"}, []string{"Apple", "Zebra"}},
	} {
		var body struct {
			Total int
			Items []struct{ ID, Title string }
		}
		if err := json.Unmarshal(check.response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		var ids, titles []string
		for _, item := range body.Items {
			ids = append(ids, item.ID)
			titles = append(titles, item.Title)
		}
		if check.response.Code != http.StatusOK || body.Total != 2 || !slices.Equal(ids, check.ids) || !slices.Equal(titles, check.titles) {
			t.Fatalf("status=%d body=%s", check.response.Code, check.response.Body.String())
		}
	}
}
