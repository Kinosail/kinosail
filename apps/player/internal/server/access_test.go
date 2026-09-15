package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestViewerContentDefaultsFailClosed(t *testing.T) {
	t.Parallel()
	item := library.Item{Library: "Movies", Rating: "PG"}
	if canView(viewerProfile{}, item) {
		t.Fatal("new Viewer can access a library before it is granted")
	}
	viewer := viewerProfile{Libraries: []string{"all"}}
	if !canView(viewer, item) || canView(viewer, library.Item{Library: "Movies", Rating: "R"}) || canView(viewer, library.Item{Library: "Movies"}) {
		t.Fatal("new Viewer rating default is not family-safe")
	}
	viewer = viewerProfile{Rating: "family", Libraries: []string{"Movies"}}
	request := httptest.NewRequestWithContext(context.WithValue(t.Context(), viewerContextKey{}, viewer), http.MethodGet, "/", nil)
	index := memoryLibraryIndex([]library.Item{item, {ID: "mature", Library: "Movies", Rating: "R"}}, true)
	items, err := visibleLibrary(request, index)
	if err != nil || len(items) != 1 || len(index.VisibleLibrary(request)) != 1 || viewerPolicy(viewer).Owner {
		t.Fatalf("Viewer adapter = %#v, %v", items, err)
	}
}
