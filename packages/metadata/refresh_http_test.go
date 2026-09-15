package metadata

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestMetadataRefreshRejectsMutationDataBeforeLookup(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, query, body string
		readFailure       bool
	}{
		{"unknown query", "?unexpected=true", "", false},
		{"body", "", "unexpected=true", false},
		{"oversized body", "", strings.Repeat("x", 2048), false},
		{"read failure", "", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			index := &bulkIndexStub{}
			handler := RefreshHandler(index, func(context.Context, library.Item) error {
				t.Fatal("invalid request fetched metadata")
				return nil
			}, metadataRefreshFailure, http.NotFound)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/metadata/refresh"+test.query, strings.NewReader(test.body))
			if test.readFailure {
				request.Body = io.NopCloser(failingReader{})
			}
			response := httptest.NewRecorder()
			handler(response, request)
			if response.Code != http.StatusBadRequest || strings.TrimSpace(response.Body.String()) != "metadata request is invalid" {
				t.Fatalf("rejected request=%d %q", response.Code, response.Body.String())
			}
			if index.finds != 0 || index.refreshes != 0 {
				t.Fatal("invalid request reached library operations")
			}
		})
	}
}

func TestMetadataRefreshPreservesOperationOrderAndFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, kind        string
		found             bool
		fetchErr, scanErr error
		status            int
		events            []string
	}{
		{"missing", "", false, nil, nil, http.StatusNotFound, []string{"find:" + bulkIDOne}},
		{"wrong kind", "audio", true, nil, nil, http.StatusNotFound, []string{"find:" + bulkIDOne}},
		{"fetch failed", "video", true, errors.New("provider failed"), nil, http.StatusBadGateway, []string{"find:" + bulkIDOne, "fetch"}},
		{"refresh failed", "video", true, nil, errors.New("scan failed"), http.StatusBadGateway, []string{"find:" + bulkIDOne, "fetch", "refresh"}},
		{"success", "video", true, nil, nil, http.StatusSeeOther, []string{"find:" + bulkIDOne, "fetch", "refresh"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			events := []string{}
			index := &bulkIndexStub{items: map[string]library.Item{}, refreshErr: test.scanErr, events: &events}
			item := library.Item{ID: bulkIDOne, Kind: test.kind}
			if test.found {
				index.items[bulkIDOne] = item
			}
			fetch := func(ctx context.Context, got library.Item) error {
				if ctx != t.Context() || !reflect.DeepEqual(got, item) {
					t.Fatal("fetch received different context or item")
				}
				events = append(events, "fetch")
				return test.fetchErr
			}
			handler := RefreshHandler(index, fetch, metadataRefreshFailure, http.NotFound)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/metadata/refresh", nil)
			request.SetPathValue("id", bulkIDOne)
			response := httptest.NewRecorder()
			handler(response, request)
			assertMetadataRefreshResult(t, response, test.status, events, test.events)
		})
	}
}

func metadataRefreshFailure(writer http.ResponseWriter, _ *http.Request, message string, status int) {
	http.Error(writer, message, status)
}

func assertMetadataRefreshResult(t *testing.T, response *httptest.ResponseRecorder, status int, events, want []string) {
	t.Helper()
	if response.Code != status || !reflect.DeepEqual(events, want) {
		t.Fatalf("status=%d events=%q, want %d %q", response.Code, events, status, want)
	}
	if status == http.StatusSeeOther && response.Header().Get("Location") != "/watch/"+bulkIDOne {
		t.Fatalf("redirect=%q", response.Header().Get("Location"))
	}
	if status == http.StatusBadGateway && strings.TrimSpace(response.Body.String()) != "metadata provider unavailable" {
		t.Fatalf("failure body=%q", response.Body.String())
	}
}
