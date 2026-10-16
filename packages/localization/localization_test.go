package localization

import (
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
	languages := []Language{{Tag: "en", Name: "English", Direction: "ltr"}, {Tag: "es", Name: "Español", Direction: "ltr"}, {Tag: "ar", Name: "العربية", Direction: "rtl"}, {Tag: "fr", Name: "Français", Direction: "ltr"}}
	messages := []Message{{ID: "Automatic (browser)", Other: "Automatic (browser)"}, {ID: "Language", Other: "Language"}, {ID: "Save", Other: "Save"}, {ID: "Play", Other: "Play"}}
	return NewCatalog(languages, func() (*i18n.Bundle, []Message) { return bundle, messages })
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

func TestCatalogLocalizationAndHTMLTranslation(t *testing.T) {
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
	want := `<html lang="en"><button aria-label="Reproducir {{.Title}}" title="Reproducir">Reproducir</button><p>Display</p>`
	if got := catalog.translateBody(source, "es"); got != want {
		t.Fatalf("translated source = %q; want %q", got, want)
	}
	if got := catalog.translateBody(`<p aria-label="Play>Play`, "es"); got != `<p aria-label="Play>Reproducir` {
		t.Fatalf("malformed source translation = %q", got)
	}
	if got := catalog.translateBody(`{{broken`, "es"); got != `{{broken` {
		t.Fatalf("malformed template changed to %q", got)
	}
	if got := catalog.translateBody(`<broken`, "es"); got != `<broken` {
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
