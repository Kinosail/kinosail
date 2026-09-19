package httpguard

import (
	"bytes"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCSRFTemplateSourceAndExecution(t *testing.T) {
	t.Parallel()
	view := NewCSRFTemplate("forms", `<head></head><form></form><form method="get"></form><form method="post"></form><FORM METHOD='POST'></FORM>`, func(string) template.HTML { return "icon" })
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: "session-secret"})
	response := httptest.NewRecorder()
	if err := ExecuteCSRFTemplate(view, response, request, nil); err != nil {
		t.Fatal(err)
	}
	body := response.Body.String()
	if got := strings.Count(body, `name="_csrf"`); got != 2 {
		t.Fatalf("CSRF field count = %d, want two POST forms", got)
	}
	if !strings.Contains(body, `<meta name="kinosail-csrf" content="`+CSRFToken("session-secret")) {
		t.Fatalf("CSRF meta marker missing: %s", body)
	}
}

func TestCSRFParseFunctionsRemainSafeBeforeExecution(t *testing.T) {
	t.Parallel()
	functions := CSRFParseFuncs(func(string) template.HTML { return "icon" })
	for _, name := range []string{"csrfField", "csrfMeta"} {
		function := functions[name].(func() template.HTML)
		if got := function(); got != "" {
			t.Fatalf("parse placeholder %s=%q", name, got)
		}
	}
}

func TestExecuteCSRFTemplateDoesNotWriteAfterCloneFailure(t *testing.T) {
	t.Parallel()
	view := NewCSRFTemplate("already-executed", "<head></head><form method=post></form>", func(string) template.HTML { return "icon" })
	var initial bytes.Buffer
	if err := view.Execute(&initial, nil); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	if err := ExecuteCSRFTemplate(view, response, request, nil); err == nil {
		t.Fatal("executed template unexpectedly cloned")
	}
	if response.Body.Len() != 0 {
		t.Fatal("failed template clone wrote response content")
	}
}
