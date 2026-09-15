package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail-dashboard/internal/dashboard"
)

func TestStrictJSONRejectsInvalidMutationWithoutSideEffects(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	large := `{"name":"` + strings.Repeat("x", 33<<10) + `","url":"http://router.lan","expectedVersion":1}`
	tests := map[string]string{
		"unknown field":   `{"name":"Router","url":"http://router.lan","expectedVersion":1,"unexpected":true}`,
		"mis-cased field": `{"Name":"Router","url":"http://router.lan","expectedVersion":1}`,
		"duplicate key":   `{"name":"Router","name":"Switch","url":"http://router.lan","expectedVersion":1}`,
		"case collision":  `{"name":"Router","url":"http://router.lan","expectedVersion":1,"EXPECTEDVERSION":1}`,
		"malformed":       `{"name":`,
		"multiple values": `{"name":"Router"}{"url":"http://router.lan"}`,
		"array":           `[]`,
		"empty":           ``,
		"oversized":       large,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			before := app.board.Snapshot()
			response := app.request(t, http.MethodPost, "/api/v1/apps", body, &session)
			requireJSONError(t, response, http.StatusBadRequest, "invalid JSON request")
			after := app.board.Snapshot()
			if after.Version != before.Version || len(after.Apps) != len(before.Apps) {
				t.Fatalf("invalid JSON changed board from version %d to %d", before.Version, after.Version)
			}
		})
	}
}

func TestStrictJSONRejectsNullUpdateWithoutSiblingMutation(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	created := app.request(t, http.MethodPost, "/api/v1/apps", `{"name":"Router","url":"http://router.lan","favorite":false,"expectedVersion":1}`, &session)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}
	before := app.board.Snapshot()
	if len(before.Apps) != 1 {
		t.Fatalf("created applications = %d, want 1", len(before.Apps))
	}

	response := app.request(t, http.MethodPatch, "/api/v1/apps/"+before.Apps[0].ID, `{"description":null,"favorite":true,"expectedVersion":2}`, &session)
	requireJSONError(t, response, http.StatusBadRequest, "invalid JSON request")
	after := app.board.Snapshot()
	if after.Version != before.Version || len(after.Apps) != 1 || after.Apps[0].Favorite != before.Apps[0].Favorite || after.Apps[0].Description != before.Apps[0].Description {
		t.Fatalf("null update changed board: before = %#v, after = %#v", before, after)
	}
}

func TestHealthEndpointsValidateBeforeNetworkSideEffects(t *testing.T) {
	var hits atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		hits.Add(1)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	app := newTestApplication(t, Config{})
	session := app.setup(t)
	created, _, err := app.board.Create(context.Background(), dashboard.CreateInput{
		Name: "Local target", URL: target.URL, HealthURL: target.URL, CheckEnabled: true, ExpectedVersion: 1,
	}, "test")
	if err != nil {
		t.Fatal(err)
	}

	paths := []string{"/api/v1/apps/" + created.ID + "/check", "/api/v1/board/check"}
	for _, path := range paths {
		response := app.request(t, http.MethodPost, path, `{"unexpected":true}`, &session)
		requireJSONError(t, response, http.StatusBadRequest, "invalid JSON request")
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("invalid health requests made %d network calls", got)
	}

	response := app.request(t, http.MethodPost, "/api/v1/apps/"+created.ID+"/check", `{}`, &session)
	if response.Code != http.StatusOK {
		t.Fatalf("valid check status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("valid health request made %d network calls, want 1", got)
	}
}
