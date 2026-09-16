package server

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/MikeO7/kinosail/packages/localization"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

const localeCookie = "kinosail_language"

var supportedLanguages = localization.SupportedLanguages()

type languageOption struct {
	Tag       string `json:"tag"`
	Name      string `json:"name"`
	Direction string `json:"direction"`
}

var languageMatcher = func() language.Matcher {
	tags := make([]language.Tag, len(supportedLanguages))
	for index, supported := range supportedLanguages {
		tags[index] = language.MustParse(supported.Tag)
	}
	return language.NewMatcher(tags)
}()

//go:embed locales/*.json
var localeCatalogs embed.FS

type catalogMessage = localization.Message

var (
	localizationBundle, sourceMessages = loadLocalization()
	localeCatalog                      = localization.NewCatalog(localizationBundle, supportedLanguages, sourceMessages)
	localeTemplateRuntime              = localization.TemplateRuntime{
		PreferredLanguage: preferredLanguage, LanguagePreference: languagePreference, MatchLanguage: matchSupportedLanguage,
		SetLanguageHeaders: setLanguageHeaders, CSRFForRequest: csrfForRequest, CSRFTemplateFuncs: csrfTemplateFuncs,
	}
)

func loadLocalization() (*i18n.Bundle, []catalogMessage) {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	entries, err := fs.ReadDir(localeCatalogs, "locales")
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		if _, err := bundle.LoadMessageFileFS(localeCatalogs, "locales/"+entry.Name()); err != nil && !strings.HasPrefix(err.Error(), "no plural rule registered for ") {
			panic(err)
		}
	}
	var messages []catalogMessage
	for _, name := range []string{"active.en.json", "supporter.en.json"} {
		data, err := localeCatalogs.ReadFile("locales/" + name)
		if err != nil {
			panic(err)
		}
		var catalog []catalogMessage
		if err := json.Unmarshal(data, &catalog); err != nil {
			panic(err)
		}
		messages = append(messages, catalog...)
	}
	sort.Slice(messages, func(left, right int) bool { return len(messages[left].Other) > len(messages[right].Other) })
	return bundle, messages
}

type localizedTemplate = *localization.TemplateSet

func newLocalizedTemplate(name, source string) *localization.TemplateSet {
	source = strings.ReplaceAll(source, `/static/main.kinosail.bundle.js?v=7`, `/static/main.kinosail.bundle.js?v=11`)
	return localization.NewTemplateSet(name, source, "11", localeCatalog, supportedLanguages, csrfTemplateSource, csrfParseFuncs(), localeTemplateRuntime)
}

func localized(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if selected := request.URL.Query().Get("lang"); selected != "" {
			if selected, ok := matchSupportedLanguage(selected); ok {
				setLanguageCookie(writer, request, selected)
			}
		}
		next.ServeHTTP(writer, request)
	})
}

func setLanguageHeaders(writer http.ResponseWriter, tag string) {
	writer.Header().Set("Content-Language", tag)
	writer.Header().Add("Vary", "Accept-Language")
	writer.Header().Add("Vary", "Cookie")
}

func preferredLanguage(request *http.Request) string {
	if selected, ok := matchSupportedLanguage(request.URL.Query().Get("lang")); ok {
		return selected
	}
	if selected := languagePreference(request); selected != "auto" {
		return selected
	}
	return automaticLanguage(request)
}

func automaticLanguage(request *http.Request) string {
	tags, _, _ := language.ParseAcceptLanguage(request.Header.Get("Accept-Language"))
	for _, requested := range tags {
		if exact, ok := localization.ExactSupportedLanguage(supportedLanguages, requested.String()); ok {
			return automaticLanguageFallback(exact)
		}
	}
	_, index, _ := languageMatcher.Match(tags...)
	if index >= 0 && index < len(supportedLanguages) {
		return automaticLanguageFallback(supportedLanguages[index].Tag)
	}
	return "en"
}

func automaticLanguageFallback(tag string) string {
	if localization.IsTranslated(tag) {
		return tag
	}
	parent := language.Make(tag).Parent()
	if candidate, ok := localization.ExactSupportedLanguage(supportedLanguages, parent.String()); ok {
		if localization.IsTranslated(candidate) {
			return candidate
		}
	}
	return tag
}

func languagePreference(request *http.Request) string {
	if cookie, err := request.Cookie(localeCookie); err == nil {
		if selected, ok := matchSupportedLanguage(cookie.Value); ok {
			return selected
		}
	}
	return "auto"
}

func matchSupportedLanguage(value string) (string, bool) {
	if value == "" || value == "auto" {
		return "", false
	}
	tag, err := language.Parse(value)
	if err != nil {
		return "", false
	}
	_, index, confidence := languageMatcher.Match(tag)
	if confidence == language.No || index < 0 || index >= len(supportedLanguages) {
		return "", false
	}
	return supportedLanguages[index].Tag, true
}

func languageOptions() []languageOption {
	options := make([]languageOption, len(supportedLanguages))
	for index, supported := range supportedLanguages {
		options[index] = languageOption(supported)
	}
	return options
}

func localizedError(writer http.ResponseWriter, request *http.Request, message string, status int) {
	if strings.HasPrefix(request.URL.Path, "/api/") {
		apiError(writer, errors.New(message), status)
		return
	}
	tag := preferredLanguage(request)
	setLanguageHeaders(writer, tag)
	if tag != "en" {
		if message != "invalid credentials" || !localeCatalog.Knows(message) {
			message = "Request could not be completed."
		}
	}
	message = localeCatalog.Localize(tag, message)
	http.Error(writer, message, status)
}

func localizedNotFound(writer http.ResponseWriter, request *http.Request) {
	localizedError(writer, request, "not found", http.StatusNotFound)
}

func setLanguageCookie(writer http.ResponseWriter, request *http.Request, language string) {
	//nolint:gosec // G124: plaintext localhost must retain language preferences.
	http.SetCookie(writer, &http.Cookie{Name: localeCookie, Value: language, Path: "/", MaxAge: 31536000, HttpOnly: true, Secure: secureRequest(request), SameSite: http.SameSiteLaxMode})
}

func clearLanguageCookie(writer http.ResponseWriter, request *http.Request) {
	//nolint:gosec // G124: plaintext localhost must be able to expire this preference.
	http.SetCookie(writer, &http.Cookie{Name: localeCookie, Path: "/", MaxAge: -1, HttpOnly: true, Secure: secureRequest(request), SameSite: http.SameSiteLaxMode})
}

func selectLanguage(writer http.ResponseWriter, request *http.Request, selected string) (string, bool) {
	if selected == "auto" {
		clearLanguageCookie(writer, request)
		return "auto", true
	}
	matched, ok := matchSupportedLanguage(selected)
	if !ok {
		return "", false
	}
	setLanguageCookie(writer, request, matched)
	return matched, true
}

func saveWebLanguage(writer http.ResponseWriter, request *http.Request) {
	if _, ok := selectLanguage(writer, request, request.FormValue("language")); !ok {
		localizedError(writer, request, "language is invalid", http.StatusBadRequest)
		return
	}
	destination := "/"
	if referer, err := url.Parse(request.Referer()); err == nil && referer.Host == request.Host && strings.HasPrefix(referer.Path, "/") {
		destination = referer.RequestURI()
	}
	//nolint:gosec // The redirect is restricted to a parsed same-host absolute path above.
	http.Redirect(writer, request, destination, http.StatusSeeOther)
}

func apiLanguage(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Language string `json:"language"`
	}
	if !readJSON(writer, request, &input) {
		return
	}
	preference, ok := selectLanguage(writer, request, input.Language)
	if !ok {
		apiError(writer, apiContractError("language is invalid"), http.StatusBadRequest)
		return
	}
	effective := preference
	if preference == "auto" {
		effective = automaticLanguage(request)
	}
	writeJSON(writer, map[string]string{"language": effective, "languagePreference": preference}, http.StatusOK)
}
