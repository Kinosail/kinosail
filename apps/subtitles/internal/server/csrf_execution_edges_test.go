package server

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCSRFParsePlaceholdersRemainEmpty(t *testing.T) {
	t.Parallel()
	functions := csrfParseFuncs()
	for _, name := range []string{"csrfField", "csrfMeta"} {
		function := functions[name].(func() template.HTML)
		if got := function(); got != "" {
			t.Fatalf("parse placeholder %s=%q", name, got)
		}
	}
}

func TestCSRFExecutedTemplateCannotBeCloned(t *testing.T) {
	t.Parallel()
	view := newCSRFTemplate("already-executed", "<head></head><form method=post></form>")
	var initial bytes.Buffer
	if err := view.Execute(&initial, nil); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	if err := executeCSRFTemplate(view, response, request, nil); err == nil {
		t.Fatal("executed template unexpectedly cloned")
	}
	if response.Body.Len() != 0 {
		t.Fatal("failed template clone wrote response content")
	}
}

func TestCSRFFieldsOnlyProtectPostForms(t *testing.T) {
	t.Parallel()
	view := newCSRFTemplate("form-methods", `<head></head><form></form><form method="get"></form><form data-method="post"></form><form method="post"></form><FORM METHOD='POST'></FORM><form method=post class="save"></form>`)
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: "session-secret"})
	if err := executeCSRFTemplate(view, response, request, nil); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(response.Body.String(), `name="_csrf"`); got != 3 {
		t.Fatalf("CSRF field count = %d, want one in each POST form", got)
	}
}
