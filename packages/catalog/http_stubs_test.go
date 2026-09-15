package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

type collectionIndexStub struct {
	items       []library.Item
	visible     library.Item
	found       bool
	reads       int
	itemLookups int
}

func (stub *collectionIndexStub) VisibleLibrary(*http.Request) []library.Item {
	stub.reads++
	return stub.items
}

func (stub *collectionIndexStub) VisibleItem(*http.Request, string) (library.Item, bool) {
	stub.itemLookups++
	return stub.visible, stub.found
}

type collectionProgressStub struct{ calls int }

func (stub *collectionProgressStub) ClientItems(_ *http.Request, items []library.Item) any {
	stub.calls++
	return items
}

type collectionStoreStub struct {
	names                     []string
	selected                  []library.Item
	createErr, saveErr, error error
	creates, saves, deletes   int
	lastName                  string
	lastIncluded              bool
}

func (stub *collectionStoreStub) CollectionNames([]library.Item) []string { return stub.names }

func (stub *collectionStoreStub) CollectionSummaries([]library.Item) any { return []string{"summary"} }

func (stub *collectionStoreStub) Collection(string, []library.Item) []library.Item {
	return stub.selected
}

func (stub *collectionStoreStub) CreateCollection(_ context.Context, name string) error {
	stub.creates++
	stub.lastName = name
	return stub.createErr
}

func (stub *collectionStoreStub) SetCollection(_ context.Context, name string, _ library.Item, included bool) error {
	stub.saves++
	stub.lastName, stub.lastIncluded = name, included
	return stub.saveErr
}

func (stub *collectionStoreStub) DeleteCollection(_ context.Context, name string, _ []library.Item) error {
	stub.deletes++
	stub.lastName = name
	return stub.error
}

type listIndexStub struct {
	item  library.Item
	found bool
	calls int
}

func (stub *listIndexStub) VisibleItem(*http.Request, string) (library.Item, bool) {
	stub.calls++
	return stub.item, stub.found
}

type listMutationStub struct {
	calls int
	err   error
}

func (stub *listMutationStub) Create(context.Context, string, string, ...string) error {
	stub.calls++
	return stub.err
}

func (stub *listMutationStub) CreateSmart(context.Context, string, string, PlaylistRule) error {
	stub.calls++
	return stub.err
}

func (stub *listMutationStub) SetPlaylist(context.Context, string, string, string, bool) error {
	stub.calls++
	return stub.err
}

func (stub *listMutationStub) DeletePlaylist(context.Context, string, string) error {
	stub.calls++
	return stub.err
}

func (stub *listMutationStub) SetListed(context.Context, string, string, bool) error {
	stub.calls++
	return stub.err
}

func listRequest(target, body string) *http.Request {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}
