package localization

import (
	"testing"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// Real app catalogs use display text as message IDs. Their browser journeys
// cannot cover semantic IDs or translations that match a later source phrase.
func TestSourceTranslationPreservesMessageOrderAndUnknownFallback(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	for _, entry := range []struct {
		tag      language.Tag
		messages []*i18n.Message
	}{
		{language.English, []*i18n.Message{{ID: "first", Other: "One"}, {ID: "second", Other: "Two"}}},
		{language.Spanish, []*i18n.Message{{ID: "first", Other: "Two"}, {ID: "second", Other: "Dos"}}},
	} {
		if err := bundle.AddMessages(entry.tag, entry.messages...); err != nil {
			t.Fatal(err)
		}
	}
	catalog := NewCatalog(bundle, []Language{{Tag: "en"}, {Tag: "es"}},
		[]Message{{ID: "first", Other: "One"}, {ID: "second", Other: "Two"}})
	for _, entry := range []struct{ tag, want string }{
		{"en", `<p title="One Two">One Two {{.Title}}</p>`},
		{"es", `<p title="Dos Dos">Dos Dos {{.Title}}</p>`},
		{"fr", `<p title="first second">first second {{.Title}}</p>`},
	} {
		for range 2 {
			if got := catalog.TranslateSource(`<p title="One Two">One Two {{.Title}}</p>`, entry.tag); got != entry.want {
				t.Errorf("%s: got %q; want %q", entry.tag, got, entry.want)
			}
		}
	}
}
