package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestExternalImportPreviewAndCommit(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	content := "Media:\n  - Jellyfin:\n      href: http://jellyfin.local:8096\n"
	previewResponse := app.request(t, http.MethodPost, "/api/v1/import/preview", `{"source":"homepage","content":"Media:\n  - Jellyfin:\n      href: http://jellyfin.local:8096\n"}`, &session)
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", previewResponse.Code, previewResponse.Body.String())
	}
	var preview struct {
		Apps []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"apps"`
	}
	decodeBody(t, previewResponse.Body, &preview)
	if len(preview.Apps) != 1 || preview.Apps[0].Name != "Jellyfin" || preview.Apps[0].URL != "http://jellyfin.local:8096" {
		t.Fatalf("preview = %+v", preview)
	}

	board := app.board.Snapshot()
	commitBody, _ := json.Marshal(map[string]any{"source": "homepage", "content": content, "expectedVersion": board.Version})
	commitResponse := app.request(t, http.MethodPost, "/api/v1/import/external", string(commitBody), &session)
	if commitResponse.Code != http.StatusOK || len(app.board.Snapshot().Apps) != 1 {
		t.Fatalf("commit status = %d, body = %s, board = %+v", commitResponse.Code, commitResponse.Body.String(), app.board.Snapshot())
	}
}

func TestExternalImportRejectsUnknownFieldsBeforeParsing(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	response := app.request(t, http.MethodPost, "/api/v1/import/preview", `{"source":"homepage","content":"Media: []","unexpected":true}`, &session)
	requireJSONError(t, response, http.StatusBadRequest, "invalid JSON request")
}
