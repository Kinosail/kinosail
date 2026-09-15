package localization

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func testCatalog(t *testing.T) *Catalog {
	t.Helper()
	bundle := i18n.NewBundle(language.English)
	if err := bundle.AddMessages(language.English,
		&i18n.Message{ID: "Language", Other: "Language"},
		&i18n.Message{ID: "Automatic (browser)", Other: "Automatic (browser)"},
		&i18n.Message{ID: "Save", Other: "Save"},
		&i18n.Message{ID: "Play", Other: "Play"},
	); err != nil {
		t.Fatal(err)
	}
	if err := bundle.AddMessages(language.Spanish,
		&i18n.Message{ID: "Language", Other: "Idioma"},
		&i18n.Message{ID: "Automatic (browser)", Other: "Automático (navegador)"},
		&i18n.Message{ID: "Save", Other: "Guardar"},
		&i18n.Message{ID: "Play", Other: "Reproducir"},
	); err != nil {
		t.Fatal(err)
	}
	languages := []Language{{Tag: "en", Name: "English", Direction: "ltr"}, {Tag: "es", Name: "Español", Direction: "ltr"}, {Tag: "ar", Name: "العربية", Direction: "rtl"}}
	messages := []Message{{ID: "Automatic (browser)", Other: "Automatic (browser)"}, {ID: "Language", Other: "Language"}, {ID: "Save", Other: "Save"}, {ID: "Play", Other: "Play"}}
	return NewCatalog(bundle, languages, messages)
}

func TestSupportedLanguagesUsePlayerOrderAndDirections(t *testing.T) {
	t.Parallel()
	languages := SupportedLanguages()
	if len(languages) < 106 || languages[0].Tag != "en" || languages[1].Tag != "es" {
		t.Fatalf("unexpected Player language list: %d %#v", len(languages), languages[:2])
	}
	assertLanguagesUnique(t, languages)
	assertLanguagesValid(t, languages)
}

func assertLanguagesUnique(t *testing.T, languages []Language) {
	t.Helper()
	seen := make(map[string]struct{}, len(languages))
	for _, supported := range languages {
		if _, exists := seen[supported.Tag]; exists {
			t.Fatalf("duplicate language %q", supported.Tag)
		}
		seen[supported.Tag] = struct{}{}
	}
}

func assertLanguagesValid(t *testing.T, languages []Language) {
	t.Helper()
	for _, supported := range languages {
		if supported.Name == "" || supported.Direction != "ltr" && supported.Direction != "rtl" {
			t.Fatalf("invalid language %#v", supported)
		}
		if supported.Tag == "ar" && supported.Direction != "rtl" {
			t.Fatalf("Arabic direction = %q", supported.Direction)
		}
	}
}

func TestExactSupportedLanguageRequiresAnExactValidTag(t *testing.T) {
	t.Parallel()
	languages := []Language{{Tag: "en", Name: "English", Direction: "ltr"}, {Tag: "pt-BR", Name: "Português", Direction: "ltr"}}
	for _, test := range []struct {
		input string
		want  string
		ok    bool
	}{{"pt-br", "pt-BR", true}, {"en", "en", true}, {"pt", "", false}, {"not a tag", "", false}, {"", "", false}} {
		got, ok := ExactSupportedLanguage(languages, test.input)
		if got != test.want || ok != test.ok {
			t.Errorf("ExactSupportedLanguage(%q) = %q, %v; want %q, %v", test.input, got, ok, test.want, test.ok)
		}
	}
}

func TestTranslatedCatalogSet(t *testing.T) {
	t.Parallel()
	if !IsTranslated("en") || !IsTranslated("es") || IsTranslated("es-MX") || IsTranslated("") {
		t.Fatal("translated language set does not match the Player catalog")
	}
}

func TestCatalogLocalizationAndHTMLTranslation(t *testing.T) { //nolint:cyclop // One contract test verifies all localized catalog surfaces.
	t.Parallel()
	catalog := testCatalog(t)
	if got := catalog.Localize("es", "Play"); got != "Reproducir" {
		t.Fatalf("Spanish Play = %q", got)
	}
	if got := catalog.Localize("fr", "Play"); got != "Play" {
		t.Fatalf("missing translation = %q", got)
	}
	if !catalog.Knows("Play") || catalog.Knows("Missing") {
		t.Fatal("message lookup is incorrect")
	}
	if catalog.Direction("ar") != "rtl" || catalog.Direction("missing") != "ltr" {
		t.Fatal("language direction is incorrect")
	}

	source := `<html lang="en"><button aria-label="Play {{.Title}}" title="Play">Play</button><p>Display</p>`
	want := `<html lang="es" dir="ltr"><button aria-label="Reproducir {{.Title}}" title="Reproducir">Reproducir</button><p>Display</p>`
	if got := catalog.TranslateSource(source, "es"); got != want {
		t.Fatalf("translated source = %q; want %q", got, want)
	}
	for _, tag := range []string{"es", "ar"} {
		source := `<html lang="en" data-theme="dark"><body></body></html>`
		want := `<html lang="` + tag + `" dir="` + catalog.Direction(tag) + `" data-theme="dark"><body></body></html>`
		if got := catalog.TranslateSource(source, tag); got != want {
			t.Fatalf("HTML attributes = %q; want %q", got, want)
		}
	}
	if got := catalog.TranslateSource(`<p aria-label="Play>Play`, "es"); got != `<p aria-label="Play>Reproducir` {
		t.Fatalf("malformed source translation = %q", got)
	}
	if got := catalog.TranslateSource(`{{broken`, "es"); got != `{{broken` {
		t.Fatalf("malformed template changed to %q", got)
	}
	if got := catalog.TranslateSource(`<broken`, "es"); got != `<broken` {
		t.Fatalf("malformed tag changed to %q", got)
	}
}

func TestCatalogPreservesWordBoundariesAndBrokenAttributes(t *testing.T) {
	t.Parallel()
	catalog := testCatalog(t)
	if got := catalog.translateText("Play Playback Replay Play.", "es"); got != "Reproducir Playback Replay Reproducir." {
		t.Fatalf("boundary translation = %q", got)
	}
	if got := catalog.translateTag(`<input title="Play" placeholder="Play {{.Name}}" aria-label="Play">`, "es"); strings.Count(got, "Reproducir") != 3 {
		t.Fatalf("attribute translation = %q", got)
	}
	if got := catalog.translateTag(`<input title="Play>`, "es"); got != `<input title="Play>` {
		t.Fatalf("broken attribute changed to %q", got)
	}
	if got := catalog.translateTemplateText(`Play {{broken`, "es"); got != `Reproducir {{broken` {
		t.Fatalf("broken template translation = %q", got)
	}
}

func TestPickerEscapesValuesAndSelectsPreference(t *testing.T) {
	t.Parallel()
	catalog := testCatalog(t)
	full := string(catalog.PickerHTML("es", "es", `<token>`, false))
	for _, expected := range []string{`aria-label="Idioma"`, `value="&lt;token&gt;"`, `<option value="es" selected>Español</option>`, `>Guardar</button>`} {
		if !strings.Contains(full, expected) {
			t.Fatalf("full picker does not contain %q: %s", expected, full)
		}
	}
	if strings.Contains(full, `class="mode"`) {
		t.Fatal("full picker contains compact link")
	}
	compact := string(catalog.PickerHTML("en", "auto", "", true))
	if !strings.Contains(compact, `<option value="auto" selected>`) || !strings.Contains(compact, `class="mode"`) || strings.Contains(compact, `_csrf`) {
		t.Fatalf("compact picker = %s", compact)
	}
}

func TestTemplateRuntimeExecutesLocalizedViews(t *testing.T) {
	t.Parallel()
	catalog := testCatalog(t)
	views := map[string]*template.Template{
		"es": template.Must(template.New("page").Funcs(template.FuncMap{
			"languagePicker": func() template.HTML { return "" },
			"token":          func() string { return "" },
		}).Parse(`{{define "named"}}named {{languagePicker}} {{token}}{{end}}page {{languagePicker}} {{token}}`)),
	}
	runtime := TemplateRuntime{
		PreferredLanguage:  func(*http.Request) string { return "es" },
		LanguagePreference: func(*http.Request) string { return "auto" },
		MatchLanguage:      func(value string) (string, bool) { return value, value == "es" },
		SetLanguageHeaders: func(writer http.ResponseWriter, tag string) { writer.Header().Set("Content-Language", tag) },
		CSRFForRequest:     func(*http.Request) string { return "csrf" },
		CSRFTemplateFuncs: func(*http.Request) template.FuncMap {
			return template.FuncMap{"token": func() string { return "safe" }}
		},
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?lang=es", nil)
	response := httptest.NewRecorder()
	if err := runtime.ExecuteTemplate(views, catalog, response, request, "", nil); err != nil {
		t.Fatal(err)
	}
	if response.Header().Get("Content-Language") != "es" || !strings.Contains(response.Body.String(), `option value="es" selected`) || !strings.Contains(response.Body.String(), "safe") {
		t.Fatalf("localized page = %q", response.Body.String())
	}

	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil)
	response = httptest.NewRecorder()
	if err := runtime.ExecuteTemplate(views, catalog, response, request, "named", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response.Body.String(), "named") || strings.Contains(response.Body.String(), `class="mode"`) {
		t.Fatalf("named page = %q", response.Body.String())
	}
}

func TestTemplateRuntimeReportsCloneFailure(t *testing.T) {
	t.Parallel()
	catalog := testCatalog(t)
	view := template.Must(template.New("page").Parse("page"))
	if err := view.Execute(&strings.Builder{}, nil); err != nil {
		t.Fatal(err)
	}
	runtime := TemplateRuntime{
		PreferredLanguage:  func(*http.Request) string { return "es" },
		LanguagePreference: func(*http.Request) string { return "auto" },
		MatchLanguage:      func(string) (string, bool) { return "", false },
		SetLanguageHeaders: func(http.ResponseWriter, string) {},
		CSRFForRequest:     func(*http.Request) string { return "" },
		CSRFTemplateFuncs:  func(*http.Request) template.FuncMap { return nil },
	}
	err := runtime.ExecuteTemplate(map[string]*template.Template{"es": view}, catalog, httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), "", nil)
	if err == nil {
		t.Fatal("expected clone failure")
	}
}

func TestNewTemplateSetUsesPlayerScriptPolicy(t *testing.T) { //nolint:cyclop // One policy test verifies all allowed template script forms.
	t.Parallel()
	catalog := testCatalog(t)
	runtime := TemplateRuntime{
		PreferredLanguage:  func(*http.Request) string { return "es" },
		LanguagePreference: func(*http.Request) string { return "auto" },
		MatchLanguage:      func(string) (string, bool) { return "", false },
		SetLanguageHeaders: func(http.ResponseWriter, string) {},
		CSRFForRequest:     func(*http.Request) string { return "" },
		CSRFTemplateFuncs:  func(*http.Request) template.FuncMap { return nil },
	}
	preprocessed := false
	view := NewTemplateSet("page", `<html lang="en"><head><link rel="stylesheet" href="/static/app.css?v=cinema-1"></head><body>Play {{t "Play"}} {{languagePicker}}</body></html>`, "9", catalog, []Language{{Tag: "es"}}, func(source string) string {
		preprocessed = true
		return source
	}, nil, runtime)
	response := httptest.NewRecorder()
	if err := view.Execute(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), nil); err != nil {
		t.Fatal(err)
	}
	body := response.Body.String()
	if !preprocessed || !strings.Contains(body, `main.kinosail.bundle.js?v=9`) || !strings.Contains(body, "Reproducir") {
		t.Fatalf("compiled template = %q", body)
	}
	response = httptest.NewRecorder()
	if err := view.ExecuteTemplate(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil), "missing", nil); err == nil {
		t.Fatal("expected missing named template error")
	}
	var direct strings.Builder
	if err := view.views["es"].Execute(&direct, nil); err != nil || !strings.Contains(direct.String(), "Reproducir") || !strings.Contains(direct.String(), "language-picker") {
		t.Fatalf("stored localized template = %q, %v", direct.String(), err)
	}

	for _, source := range []string{
		`<head><link rel="stylesheet" href="/static/app.css"><script src="/static/main.kinosail.bundle.js"></script></head>`,
		`<head><link rel="stylesheet" href="/static/app.css"></head><body class="auth">`,
		`<head></head>`,
	} {
		candidate := NewTemplateSet("page", source, "9", catalog, []Language{{Tag: "es"}}, func(value string) string { return value }, nil, runtime)
		response = httptest.NewRecorder()
		if err := candidate.Execute(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), nil); err != nil {
			t.Fatal(err)
		}
		if strings.Count(response.Body.String(), "main.kinosail.bundle.js") > 1 {
			t.Fatalf("script duplicated in %q", response.Body.String())
		}
	}
}
