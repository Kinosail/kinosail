// Package subtitlelanguage owns canonical subtitle language identities and provider codes.
package subtitlelanguage

import (
	"sort"
	"strings"
	"sync"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// Provider identifies one subtitle search adapter.
type Provider string

const (
	SubDL         Provider = "subdl"
	OpenSubtitles Provider = "opensubtitles"
	SubSource     Provider = "subsource"
)

// Choice is one selectable canonical language.
type Choice struct {
	Tag       string            `json:"tag"`
	Name      string            `json:"name"`
	Common    bool              `json:"common"`
	Providers map[Provider]bool `json:"providers"`
}

const baseTags = "aa ab ae af ak am an ar as av ay az ba be bg bi bm bn bo br bs ca ce ch co cr cs cu cv cy da de dv dz ee el en eo es et eu fa ff fi fj fo fr fy ga gd gl gn gu gv ha he hi ho hr ht hu hy hz ia id ie ig ii ik io is it iu ja jv ka kg ki kj kk kl km kn ko kr ks ku kv kw ky la lb lg li ln lo lt lu lv mg mh mi mk ml mn mr ms mt my na nb nd ne ng nl nn no nr nv ny oc oj om or os pa pi pl ps pt qu rm rn ro ru rw sa sc sd se sg sh si sk sl sm sn so sq sr ss st su sv sw ta te tg th ti tk tl tn to tr ts tt tw ty ug uk ur uz ve vi vo wa wo xh yi yo za zh zu"

var (
	commonTags  = strings.Fields("en es es-419 fr de pt-BR pt-PT it nl pl ru uk tr ar fa he hi bn ur id ms vi th zh-Hans zh-Hant ja ko fil tl ta te sw ro")
	tierTwoTags = strings.Fields("cs sk hu bg el sr-Cyrl sr-Latn hr bs sl mk sq ca eu gl sv da nb nn fi is et lv lt ka hy az kk uz km my ne si mr gu pa ml kn am af")
	variantTags = []string{"es-ES", "es-419", "pt-BR", "pt-PT", "pt-MZ", "zh-Hans", "zh-Hant", "sr-Cyrl", "sr-Latn", "fil", "yue", "ckb", "ceb", "ast", "prs", "ext", "mni", "cnr", "sat", "syr", "tet", "tok", "azb"}
	once        sync.Once
	choices     []Choice
	known       map[string]Choice
)

var selectableAliases = map[string]string{"eng": "en", "spa": "es", "zh-cn": "zh-Hans", "zh-tw": "zh-Hant"}

// openSubtitlesCodes is a reviewed static snapshot of official /infos/languages codes.
var openSubtitlesCodes = map[string]string{
	"ab": "ab", "af": "af", "sq": "sq", "am": "am", "ar": "ar", "an": "an", "hy": "hy", "as": "as", "az": "az-az", "eu": "eu", "be": "be", "bn": "bn", "bs": "bs", "br": "br", "bg": "bg", "my": "my", "ca": "ca", "hr": "hr", "cs": "cs", "da": "da", "nl": "nl", "en": "en", "eo": "eo", "et": "et", "fi": "fi", "fr": "fr", "gd": "gd", "gl": "gl", "ka": "ka", "de": "de", "el": "el", "he": "he", "hi": "hi", "hu": "hu", "is": "is", "ig": "ig", "id": "id", "ia": "ia", "ga": "ga", "it": "it", "ja": "ja", "kn": "kn", "kk": "kk", "km": "km", "ko": "ko", "ku": "ku", "lv": "lv", "lt": "lt", "lb": "lb", "mk": "mk", "ms": "ms", "ml": "ml", "mr": "mr", "mn": "mn", "nv": "nv", "ne": "ne", "se": "se", "no": "no", "oc": "oc", "or": "or", "fa": "fa", "pl": "pl", "ps": "ps", "ro": "ro", "ru": "ru", "sr": "sr", "sd": "sd", "si": "si", "sk": "sk", "sl": "sl", "so": "so", "es": "es", "sw": "sw", "sv": "sv", "tl": "tl", "ta": "ta", "tt": "tt", "te": "te", "th": "th", "tr": "tr", "tk": "tk", "uk": "uk", "ur": "ur", "uz": "uz", "vi": "vi", "cy": "cy",
	"es-ES": "sp", "es-419": "ea", "pt": "pt-pt", "pt-PT": "pt-pt", "pt-BR": "pt-br", "zh": "zh-cn", "zh-Hans": "zh-cn", "zh-Hant": "zh-tw", "yue": "zh-ca",
	"ast": "at", "prs": "pr", "ext": "ex", "mni": "ma", "cnr": "me", "pt-MZ": "pm", "sat": "sx", "syr": "sy", "tet": "tm-td", "tok": "tp", "azb": "az-zb",
}

var subDLCodes = providerCodeMap(strings.Fields("AR DA NL EN FA FI FR ID IT NO RO ES SV VI SQ AZ BE BN BS BG MY CA ZH HR CS EO ET KA DE EL KL HE HI HU IS JA KO KU LV LT MK MS ML PL PT RU SR SI SK SL TL TA TE TH TR UK UR"))

var subSourceCodes = map[string]string{
	"ab": "Abkhazian", "af": "Afrikaans", "sq": "Albanian", "am": "Amharic", "ar": "Arabic", "an": "Aragonese", "hy": "Armenian", "as": "Assamese", "az": "Azerbaijani", "eu": "Basque", "be": "Belarusian", "bn": "Bengali", "bs": "Bosnian", "pt-BR": "Brazillian Portuguese", "br": "Breton", "bg": "Bulgarian", "my": "Burmese", "ca": "Catalan", "zh": "Chinese BG code", "hr": "Croatian", "cs": "Czech", "da": "Danish", "nl": "Dutch", "en": "English", "eo": "Espranto", "et": "Estonian", "fa": "Farsi_persian", "fi": "Finnish", "fr": "French", "gd": "Gaelic", "ka": "Georgian", "de": "German", "el": "Greek", "he": "Hebrew", "hi": "Hindi", "hu": "Hungarian", "is": "Icelandic", "ig": "Igbo", "id": "Indonesian", "ia": "Interlingua", "ga": "Irish", "it": "Italian", "ja": "Japanese", "kn": "Kannada", "kk": "Kazakh", "km": "Khmer", "ko": "Korean", "ku": "Kurdish", "lv": "Latvian", "lt": "Lithuanian", "lb": "Luxembourgish", "mk": "Macedonian", "ms": "Malay", "ml": "Malayalam", "mr": "Marathi", "mn": "Mongolian", "nv": "Navajo", "ne": "Nepali", "se": "Northern Sami", "no": "Norwegian", "oc": "Occitan", "pl": "Polish", "pt": "Portuguese", "pt-PT": "Portuguese", "ps": "Pushto", "ro": "Romanian", "ru": "Russian", "sr": "Serbian", "sd": "Sindhi", "si": "Sinhala", "sk": "Slovak", "sl": "Slovenian", "so": "Somali", "es": "Spanish", "sw": "Swahili", "sv": "Swedish", "tl": "Tagalog", "ta": "Tamil", "tt": "Tatar", "te": "Telugu", "th": "Thai", "tr": "Turkish", "tk": "Turkmen", "uk": "Ukrainian", "ur": "Urdu", "uz": "Uzbek", "vi": "Vietnamese", "cy": "Welsh",
}

func initCatalog() { //nolint:gocognit // The static catalog is initialized through one explicit normalization pass.
	known = make(map[string]Choice)
	common := make(map[string]bool, len(commonTags))
	priority := make(map[string]int, len(commonTags)+len(tierTwoTags))
	for _, tag := range commonTags {
		common[tag] = true
	}
	for index, tag := range append(append([]string(nil), commonTags...), tierTwoTags...) {
		priority[tag] = index + 1
	}
	tags := append(strings.Fields(baseTags), variantTags...)
	for _, tag := range tags {
		if _, found := known[tag]; found {
			continue
		}
		name := display.English.Tags().Name(language.Make(tag))
		if override := variantNames[tag]; override != "" {
			name = override
		}
		choice := Choice{Tag: tag, Name: name, Common: common[tag], Providers: map[Provider]bool{SubDL: supports(tag, SubDL), OpenSubtitles: supports(tag, OpenSubtitles), SubSource: supports(tag, SubSource)}}
		known[tag] = choice
		choices = append(choices, choice)
	}
	sort.SliceStable(choices, func(i, j int) bool {
		left, right := priority[choices[i].Tag], priority[choices[j].Tag]
		if left != 0 || right != 0 {
			if left == 0 {
				return false
			}
			if right == 0 {
				return true
			}
			return left < right
		}
		return choices[i].Name < choices[j].Name
	})
}

var variantNames = map[string]string{
	"es-ES": "Spanish (Spain)", "es-419": "Spanish (Latin America)", "pt-BR": "Portuguese (Brazil)", "pt-PT": "Portuguese (Portugal)", "pt-MZ": "Portuguese (Mozambique)", "zh-Hans": "Chinese (Simplified)", "zh-Hant": "Chinese (Traditional)", "sr-Cyrl": "Serbian (Cyrillic)", "sr-Latn": "Serbian (Latin)", "fil": "Filipino", "yue": "Cantonese", "ckb": "Central Kurdish", "ceb": "Cebuano", "ast": "Asturian", "prs": "Dari", "ext": "Extremaduran", "mni": "Manipuri", "cnr": "Montenegrin", "sat": "Santali", "syr": "Syriac", "tet": "Tetum", "tok": "Toki Pona", "azb": "South Azerbaijani",
}

func providerCodeMap(codes []string) map[string]string {
	result := make(map[string]string, len(codes)+2)
	for _, code := range codes {
		result[strings.ToLower(code)] = code
	}
	result["pt-BR"], result["zh-Hant"] = "BR_PT", "ZH_BG"
	return result
}

// Catalog returns a common-first copy of every supported language identity.
func Catalog() []Choice {
	once.Do(initCatalog)
	result := make([]Choice, len(choices))
	for index, choice := range choices {
		choice.Providers = map[Provider]bool{SubDL: choice.Providers[SubDL], OpenSubtitles: choice.Providers[OpenSubtitles], SubSource: choice.Providers[SubSource]}
		result[index] = choice
	}
	return result
}

// NormalizeTag returns a canonical selectable tag from case-insensitive BCP 47 input.
func NormalizeTag(value string) (string, bool) {
	if value != strings.TrimSpace(value) || !validTagSyntax(value) {
		return "", false
	}
	if canonical := selectableAliases[strings.ToLower(value)]; canonical != "" {
		return canonical, true
	}
	once.Do(initCatalog)
	for tag := range known {
		if strings.EqualFold(tag, value) {
			return tag, true
		}
	}
	return "", false
}

// NormalizeLocal accepts selectable identities and registered local BCP 47 variants.
func NormalizeLocal(value string) (string, bool) {
	if canonical, ok := NormalizeTag(value); ok {
		return canonical, true
	}
	if value != strings.TrimSpace(value) || !validTagSyntax(value) {
		return "", false
	}
	parsed, err := language.Parse(value)
	if err != nil {
		return "", false
	}
	canonical := parsed.String()
	base, _ := parsed.Base()
	once.Do(initCatalog)
	if known[base.String()].Tag == "" {
		return "", false
	}
	return canonical, true
}

// NormalizeProvider translates only the codes documented by one provider adapter.
func NormalizeProvider(value string, provider Provider) (string, bool) {
	if value != strings.TrimSpace(value) || value == "" || len(value) > 64 {
		return "", false
	}
	var codes map[string]string
	switch provider {
	case SubDL:
		codes = subDLCodes
	case OpenSubtitles:
		codes = openSubtitlesCodes
	case SubSource:
		codes = subSourceCodes
	default:
		return "", false
	}
	for tag, code := range codes {
		if strings.EqualFold(value, code) {
			return tag, true
		}
	}
	return NormalizeLocal(value)
}

func validTagSyntax(value string) bool { //nolint:cyclop,gocognit // Tag syntax validation keeps all canonical constraints together.
	if value == "" || len(value) > 64 {
		return false
	}
	parts := strings.Split(value, "-")
	if len(parts) > 2 || len(parts[0]) < 2 || len(parts[0]) > 3 || len(parts) == 2 && (len(parts[1]) < 2 || len(parts[1]) > 4) {
		return false
	}
	for index, part := range parts {
		for _, character := range part {
			letter := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
			if !letter && (index == 0 || character < '0' || character > '9') {
				return false
			}
		}
	}
	return true
}

// Code returns the exact provider code for a canonical tag.
func Code(tag string, provider Provider) (string, bool) {
	switch provider {
	case SubDL:
		value, ok := subDLCodes[tag]
		return value, ok
	case OpenSubtitles:
		value, ok := openSubtitlesCodes[tag]
		return value, ok
	case SubSource:
		if tag == "pt-PT" {
			return "", false
		}
		value, ok := subSourceCodes[tag]
		return value, ok
	default:
		return "", false
	}
}

func supports(tag string, provider Provider) bool {
	_, ok := Code(tag, provider)
	return ok
}
