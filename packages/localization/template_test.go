package localization

import (
	"bytes"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const sessionHeader = "X-Test-Session"

func testRuntime() TemplateRuntime {
	return TemplateRuntime{
		PreferredLanguage: func(request *http.Request) string {
			if tag := request.URL.Query().Get("lang"); tag != "" {
				return tag
			}
			return "en"
		},
		LanguagePreference: func(request *http.Request) string {
			if cookie, err := request.Cookie("language"); err == nil {
				return cookie.Value
			}
			return "auto"
		},
		MatchLanguage: func(value string) (string, bool) {
			return value, value == "es" || value == "ar" || value == "fr" || value == "en"
		},
		SetLanguageHeaders: func(writer http.ResponseWriter, tag string) { writer.Header().Set("Content-Language", tag) },
		CSRFForRequest:     func(request *http.Request) string { return request.Header.Get(sessionHeader) },
		RequestHTML: map[string]func(*http.Request) template.HTML{
			"csrfField": func(request *http.Request) template.HTML {
				return template.HTML(`<input name="_csrf" value="` + template.HTMLEscapeString(request.Header.Get(sessionHeader)) + `">`) //nolint:gosec // Test value is escaped.
			},
			"csrfMeta": func(request *http.Request) template.HTML {
				return template.HTML(`<meta name="csrf" content="` + template.HTMLEscapeString(request.Header.Get(sessionHeader)) + `">`) //nolint:gosec // Test value is escaped.
			},
		},
	}
}

func identitySource(source string) string { return source }

func static(source string) func() string { return func() string { return source } }

const requestPage = `<!doctype html><html lang="en" data-theme="dark"><head>{{csrfMeta}}</head><body><h1>{{.}}</h1><p title="Play">{{t "Play"}}</p>{{languagePicker}}<form method="post">{{csrfField}}<button>Save</button></form>{{range 2}}<form method="post">{{csrfField}}</form>{{end}}</body></html>`

func renderPage(t *testing.T, view *TemplateSet, target, session string, data any) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	if session != "" {
		request.Header.Set(sessionHeader, session)
	}
	response := httptest.NewRecorder()
	if err := view.Execute(response, request, data); err != nil {
		t.Fatalf("render %s: %v", target, err)
	}
	return response
}

func TestTemplateSetRendersEachRequestsOwnValues(t *testing.T) {
	t.Parallel()
	view := NewTemplateSet("page", static(requestPage), "9", testCatalog(t), identitySource, nil, testRuntime())
	for _, test := range []struct {
		target, session, lang, dir, selected, play, save string
		compact                                          bool
	}{
		{"/?lang=es", "alpha&<", "es", "ltr", "es", "Reproducir", "Guardar", true},
		{"/settings?lang=ar", "", "ar", "rtl", "ar", "Play", "Save", false},
		{"/settings?lang=fr", "bravo", "fr", "ltr", "fr", "Play", "Save", false},
		{"/settings", "charlie", "en", "ltr", "auto", "Play", "Save", false},
	} {
		body := renderPage(t, view, "https://kinosail.test"+test.target, test.session, "<b>title</b>").Body.String()
		escaped := template.HTMLEscapeString(test.session)
		for _, want := range []string{
			`<html lang="` + test.lang + `" dir="` + test.dir + `" data-theme="dark"><head>`,
			`<h1>&lt;b&gt;title&lt;/b&gt;</h1>`,
			`<p title="` + test.play + `">` + test.play + `</p>`,
			`<option value="` + test.selected + `" selected>`,
			`<button>` + test.save + `</button>`,
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s does not contain %q: %s", test.target, want, body)
			}
		}
		if strings.Contains(body, "\x00") {
			t.Fatalf("%s leaked a placeholder: %q", test.target, body)
		}
		if got := strings.Count(body, `<input name="_csrf" value="`+escaped+`">`); got != 3 {
			t.Fatalf("%s rendered %d session fields: %s", test.target, got, body)
		}
		if !strings.Contains(body, `<meta name="csrf" content="`+escaped+`">`) {
			t.Fatalf("%s does not contain its session meta: %s", test.target, body)
		}
		hidden := `<input type="hidden" name="_csrf" value="` + escaped + `">`
		if strings.Contains(body, hidden) != (test.session != "") {
			t.Fatalf("%s picker session field mismatch: %s", test.target, body)
		}
		if strings.Contains(body, `class="mode"`) != test.compact {
			t.Fatalf("%s compact picker = %v: %s", test.target, !test.compact, body)
		}
	}
}

func TestTemplateSetSelectsStoredPreferenceUnlessQueryOverrides(t *testing.T) {
	t.Parallel()
	view := NewTemplateSet("page", static(requestPage), "9", testCatalog(t), identitySource, nil, testRuntime())
	for target, selected := range map[string]string{"/settings": "ar", "/settings?lang=es": "es"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
		request.AddCookie(&http.Cookie{Name: "language", Value: "ar", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		response := httptest.NewRecorder()
		if err := view.Execute(response, request, nil); err != nil {
			t.Fatal(err)
		}
		body := response.Body.String()
		if strings.Count(body, " selected>") != 1 || !strings.Contains(body, `<option value="`+selected+`" selected>`) {
			t.Fatalf("%s selected option is not %s: %s", target, selected, body)
		}
	}
}

func TestTemplateSetConcurrentRendersKeepSessionsSeparate(t *testing.T) {
	t.Parallel()
	view := NewTemplateSet("page", static(requestPage), "9", testCatalog(t), identitySource, nil, testRuntime())
	var wait sync.WaitGroup
	failures := make(chan string, 64)
	for worker := range 64 {
		wait.Go(func() {
			session := "session-" + string(rune('A'+worker%26)) + strings.Repeat("x", worker)
			lang := []string{"en", "es", "ar", "fr"}[worker%4]
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings?lang="+lang, nil)
			request.Header.Set(sessionHeader, session)
			response := httptest.NewRecorder()
			if err := view.Execute(response, request, nil); err != nil {
				failures <- err.Error()
				return
			}
			body := response.Body.String()
			if strings.Count(body, session+`"`) != 5 || !strings.Contains(body, `<html lang="`+lang+`"`) {
				failures <- body
			}
		})
	}
	wait.Wait()
	close(failures)
	for failure := range failures {
		t.Fatal(failure)
	}
}

func TestTemplateSetWritesNothingWhenRenderingFails(t *testing.T) {
	t.Parallel()
	funcs := template.FuncMap{"fail": func() (string, error) { return "", errors.New("render failed") }}
	view := NewTemplateSet("page", static(`<html lang="en"><body>visible {{csrfField}} {{fail}}</body></html>`), "9", testCatalog(t), identitySource, funcs, testRuntime())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?lang=es", nil)
	request.Header.Set(sessionHeader, "alpha")
	response := httptest.NewRecorder()
	if err := view.Execute(response, request, nil); err == nil {
		t.Fatal("expected render failure")
	}
	if response.Body.Len() != 0 || response.Header().Get("Content-Language") != "es" {
		t.Fatalf("failed render wrote %q with headers %v", response.Body.String(), response.Header())
	}
}

func TestTemplateSetDefersParsingUntilFirstUse(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	preprocessed := 0
	view := NewTemplateSet("page", static(`<html lang="en"><body>{{broken</body></html>`), "9", testCatalog(t), func(source string) string {
		preprocessed++
		return source
	}, nil, testRuntime())
	if preprocessed != 0 {
		t.Fatalf("construction preprocessed the source %d times", preprocessed)
	}
	if err := view.Validate(); err == nil {
		t.Fatal("broken template validated")
	}
	response := httptest.NewRecorder()
	if err := view.Execute(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), nil); err == nil || response.Body.Len() != 0 {
		t.Fatalf("broken template rendered %q, %v", response.Body.String(), err)
	}
	if preprocessed != 1 {
		t.Fatalf("source was preprocessed %d times", preprocessed)
	}
	record := logs.String()
	if strings.Count(record, `"level":"ERROR"`) != 1 || !strings.Contains(record, `"template":"page"`) || !strings.Contains(record, `"language":"en"`) {
		t.Fatalf("parse failure log = %s", record)
	}
}

func TestTemplateSetValidateRejectsUnsafeContexts(t *testing.T) {
	t.Parallel()
	view := NewTemplateSet("page", static(`<html lang="en"><body><a href="{{.}}`), "9", testCatalog(t), identitySource, nil, testRuntime())
	if err := view.Validate(); err == nil {
		t.Fatal("unterminated attribute validated")
	}
	valid := NewTemplateSet("page", static(requestPage), "9", testCatalog(t), identitySource, nil, testRuntime())
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestTemplateSetExecutesNamedTemplates(t *testing.T) {
	t.Parallel()
	view := NewTemplateSet("page", static(`{{define "named"}}<p>{{t "Play"}} {{csrfField}}</p>{{end}}<p>root</p>`), "9", testCatalog(t), identitySource, nil, testRuntime())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings?lang=es", nil)
	request.Header.Set(sessionHeader, "alpha")
	response := httptest.NewRecorder()
	if err := view.ExecuteTemplate(response, request, "named", nil); err != nil {
		t.Fatal(err)
	}
	if got := response.Body.String(); got != `<p>Reproducir <input name="_csrf" value="alpha"></p>` {
		t.Fatalf("named template = %q", got)
	}
	if err := view.ExecuteTemplate(httptest.NewRecorder(), request, "missing", nil); err == nil {
		t.Fatal("expected missing named template error")
	}
}

func TestNewTemplateSetUsesPlayerScriptPolicy(t *testing.T) {
	t.Parallel()
	catalog := testCatalog(t)
	view := NewTemplateSet("page", static(`<html lang="en"><head><link rel="stylesheet" href="/static/app.css?v=cinema-1"></head><body>Play {{t "Play"}}</body></html>`), "9", catalog, identitySource, nil, testRuntime())
	body := renderPage(t, view, "/?lang=es", "", nil).Body.String()
	if !strings.Contains(body, `main.kinosail.bundle.js?v=9`) || !strings.Contains(body, "Reproducir Reproducir") {
		t.Fatalf("compiled template = %q", body)
	}
	for _, source := range []string{
		`<head><link rel="stylesheet" href="/static/app.css"><script src="/static/main.kinosail.bundle.js"></script></head>`,
		`<head><link rel="stylesheet" href="/static/app.css"></head><body class="auth">`,
		`<head></head>`,
	} {
		candidate := NewTemplateSet("page", static(source), "9", catalog, identitySource, nil, testRuntime())
		if body := renderPage(t, candidate, "/?lang=es", "", nil).Body.String(); strings.Count(body, "main.kinosail.bundle.js") > 1 {
			t.Fatalf("script duplicated in %q", body)
		}
	}
}
