package subtitlelanguage

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"testing"
)

const expectedBaseTags = "aa ab ae af ak am an ar as av ay az ba be bg bi bm bn bo br bs ca ce ch co cr cs cu cv cy da de dv dz ee el en eo es et eu fa ff fi fj fo fr fy ga gd gl gn gu gv ha he hi ho hr ht hu hy hz ia id ie ig ii ik io is it iu ja jv ka kg ki kj kk kl km kn ko kr ks ku kv kw ky la lb lg li ln lo lt lu lv mg mh mi mk ml mn mr ms mt my na nb nd ne ng nl nn no nr nv ny oc oj om or os pa pi pl ps pt qu rm rn ro ru rw sa sc sd se sg sh si sk sl sm sn so sq sr ss st su sv sw ta te tg th ti tk tl tn to tr ts tt tw ty ug uk ur uz ve vi vo wa wo xh yi yo za zh zu"

func TestCatalogExceedsBazarrLanguageIdentityCount(t *testing.T) { //nolint:cyclop // The catalog contract enumerates all supported identity boundaries.
	catalog := Catalog()
	if len(catalog) <= 200 {
		t.Fatalf("catalog has %d identities; want more than Bazarr's 187", len(catalog))
	}
	seen := make(map[string]bool, len(catalog))
	for _, choice := range catalog {
		canonical, ok := NormalizeTag(choice.Tag)
		if seen[choice.Tag] || !ok || canonical != choice.Tag || choice.Name == "" {
			t.Fatalf("invalid catalog choice: %#v canonical=%q ok=%v", choice, canonical, ok)
		}
		seen[choice.Tag] = true
	}
	for index, tag := range commonTags {
		if catalog[index].Tag != tag || !catalog[index].Common || catalog[index].Name == "" {
			t.Fatalf("common choice %d = %#v; want %q with a name", index, catalog[index], tag)
		}
	}
	for index, tag := range tierTwoTags {
		choice := catalog[len(commonTags)+index]
		if choice.Tag != tag || choice.Common || choice.Name == "" {
			t.Fatalf("tier two choice %d = %#v; want %q", index, choice, tag)
		}
	}
}

func TestCatalogContainsEveryRegisteredTwoLetterLanguage(t *testing.T) {
	expected := strings.Fields(expectedBaseTags)
	if len(expected) != 184 {
		t.Fatalf("authoritative base set has %d tags; want 184", len(expected))
	}
	actual := make(map[string]bool, len(expected))
	for _, choice := range Catalog() {
		if len(choice.Tag) == 2 {
			actual[choice.Tag] = true
		}
	}
	for _, tag := range expected {
		if !actual[tag] {
			t.Errorf("catalog omits registered base language %q", tag)
		}
		delete(actual, tag)
	}
	for tag := range actual {
		t.Errorf("catalog has unexpected two-letter language %q", tag)
	}
}

func TestNormalizeTagIsCaseInsensitiveAndProviderSpellingsStayRejected(t *testing.T) { //nolint:cyclop // The table covers canonical and provider-specific spellings.
	for input, expected := range map[string]string{"EN": "en", "eng": "en", "spa": "es", "PT-br": "pt-BR", "zh-cn": "zh-Hans", "ZH-hant": "zh-Hant", "tl": "tl", "fil": "fil", "sh": "sh", "sr-latn": "sr-Latn"} {
		if actual, ok := NormalizeTag(input); !ok || actual != expected {
			t.Errorf("NormalizeTag(%q) = %q, %v; want %q, true", input, actual, ok, expected)
		}
	}
	for _, input := range []string{" English ", "English", "english", "pt_BR", "br_pt", "ea", "sp", "at", "pm", "zh_bg", "en-US", "xx-US"} {
		if actual, ok := NormalizeTag(input); ok {
			t.Errorf("NormalizeTag(%q) = %q, true; want rejection", input, actual)
		}
	}
	for input, expected := range map[string]string{"en-US": "en-US", "ES-419": "es-419", "pt-br": "pt-BR", "ZH-hant": "zh-Hant"} {
		if actual, ok := NormalizeLocal(input); !ok || actual != expected {
			t.Errorf("NormalizeLocal(%q) = %q, %v; want %q, true", input, actual, ok, expected)
		}
	}
	for _, input := range []string{"ea", "sp", "at", "pm", "zh_bg", "BR_PT", "Brazillian Portuguese", "Farsi_persian", "English", "pt_BR", " en"} {
		if actual, ok := NormalizeLocal(input); ok {
			t.Errorf("NormalizeLocal(%q) = %q, true; want rejection", input, actual)
		}
	}
}

func TestNormalizeProviderKeepsAliasesInsideTheirAdapter(t *testing.T) {
	tests := []struct {
		provider Provider
		input    string
		want     string
	}{
		{SubDL, "BR_PT", "pt-BR"},
		{SubDL, "ZH_BG", "zh-Hant"},
		{OpenSubtitles, "ea", "es-419"},
		{OpenSubtitles, "sp", "es-ES"},
		{OpenSubtitles, "at", "ast"},
		{OpenSubtitles, "pm", "pt-MZ"},
		{SubSource, "Brazillian Portuguese", "pt-BR"},
		{SubSource, "Farsi_persian", "fa"},
	}
	for _, test := range tests {
		if actual, ok := NormalizeProvider(test.input, test.provider); !ok || actual != test.want {
			t.Errorf("NormalizeProvider(%q, %q) = %q, %v; want %q, true", test.input, test.provider, actual, ok, test.want)
		}
	}
	for _, test := range []struct {
		provider Provider
		input    string
	}{{SubDL, "ea"}, {SubDL, "sp"}, {SubDL, "at"}, {SubDL, "pm"}, {OpenSubtitles, "ZH_BG"}, {OpenSubtitles, "BR_PT"}, {SubSource, "ea"}, {SubSource, "zh_bg"}, {SubSource, "English_US"}} {
		if actual, ok := NormalizeProvider(test.input, test.provider); ok {
			t.Errorf("NormalizeProvider(%q, %q) = %q, true; want rejection", test.input, test.provider, actual)
		}
	}
}

func TestProviderCodesAreExactAndDoNotBroadenVariants(t *testing.T) {
	tests := []struct {
		tag      string
		provider Provider
		want     string
	}{
		{"pt-BR", SubDL, "BR_PT"},
		{"pt-BR", OpenSubtitles, "pt-br"},
		{"pt-BR", SubSource, "Brazillian Portuguese"},
		{"pt-PT", OpenSubtitles, "pt-pt"},
		{"zh-Hans", OpenSubtitles, "zh-cn"},
		{"zh-Hant", SubDL, "ZH_BG"},
		{"zh-Hant", OpenSubtitles, "zh-tw"},
		{"es-419", OpenSubtitles, "ea"},
		{"es-ES", OpenSubtitles, "sp"},
	}
	for _, test := range tests {
		if actual, ok := Code(test.tag, test.provider); !ok || actual != test.want {
			t.Errorf("Code(%q, %q) = %q, %v; want %q, true", test.tag, test.provider, actual, ok, test.want)
		}
	}
	for _, test := range []struct {
		tag      string
		provider Provider
	}{
		{"pt-PT", SubDL},
		{"pt-PT", SubSource},
		{"zh-Hans", SubDL},
		{"zh-Hans", SubSource},
		{"zh-Hant", SubSource},
		{"es-419", SubDL},
		{"es-419", SubSource},
		{"sr-Latn", SubDL},
		{"sr-Latn", OpenSubtitles},
		{"sr-Latn", SubSource},
		{"aa", SubDL},
	} {
		if code, ok := Code(test.tag, test.provider); ok {
			t.Errorf("Code(%q, %q) = %q, true; want unsupported", test.tag, test.provider, code)
		}
	}
}

func TestProviderMappingSnapshotsHaveReviewedDigestsAndCounts(t *testing.T) {
	want := map[Provider]string{
		SubDL:         "73f9de19edf494eaff9c69486d9ddad17dd1bf609980c25bba49c50868c88ee9",
		OpenSubtitles: "aad5bc5d7544cf960a25054992493fd1174e9002e44d5472335cc080b6d9d790",
		SubSource:     "bdd7fb06e317140595933b14c30330517e1bd264dcf42abee8f91c90433cafbb",
	}
	wantCount := map[Provider]int{SubDL: 59, OpenSubtitles: 106, SubSource: 87}
	for _, provider := range []Provider{SubDL, OpenSubtitles, SubSource} {
		entries := make([]string, 0)
		for _, choice := range Catalog() {
			if code, ok := Code(choice.Tag, provider); ok {
				entries = append(entries, choice.Tag+"="+code)
			}
		}
		sort.Strings(entries)
		if len(entries) != wantCount[provider] {
			t.Fatalf("%s mapping count = %d; want the %d reviewed codes", provider, len(entries), wantCount[provider])
		}
		got := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(entries, "\n"))))
		if got != want[provider] {
			t.Errorf("%s mapping digest = %s", provider, got)
		}
	}
}
