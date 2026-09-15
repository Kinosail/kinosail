package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/text/language"

	"github.com/MikeO7/kinosail/packages/localization"
)

// LanguagePickerShowsEverySupportedLocale verifies automatic selection and every native locale name.
func LanguagePickerShowsEverySupportedLocale(t *testing.T, handler http.Handler, languages []localization.Language) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/language", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("language page status = %d", response.Code)
	}
	body := response.Body.String()
	if got := strings.Count(body, `<option value="`); got != len(languages)+1 {
		t.Fatalf("language picker option count = %d, want %d", got, len(languages)+1)
	}
	if !strings.Contains(body, `<option value="auto" selected>`) {
		t.Fatal("language picker does not select automatic detection")
	}
	for _, supported := range languages {
		option := `<option value="` + supported.Tag + `">`
		if !strings.Contains(body, option) {
			t.Fatalf("language picker is missing %s", supported.Tag)
		}
		if !strings.Contains(body, option+supported.Name+`</option>`) {
			t.Fatalf("language picker does not show the native name for %s", supported.Tag)
		}
	}
}

// AutomaticDetectionCoversEverySupportedLanguage checks detection and rendered language selection.
func AutomaticDetectionCoversEverySupportedLanguage(t *testing.T, handler http.Handler, languages []localization.Language, automaticLanguage func(*http.Request) string) {
	t.Helper()
	for _, supported := range languages {
		supported := supported
		t.Run(supported.Tag, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", nil)
			request.Header.Set("Accept-Language", supported.Tag)
			expected := automaticLanguage(request)
			wantBase, _ := language.Make(supported.Tag).Base()
			gotBase, _ := language.Make(expected).Base()
			if gotBase != wantBase {
				t.Fatalf("Accept-Language %s detected %s, want base %s", supported.Tag, expected, wantBase)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK || response.Header().Get("Content-Language") != expected || !strings.Contains(response.Body.String(), `<html lang="`+expected+`"`) || !strings.Contains(response.Body.String(), `<option value="auto" selected>`) {
				t.Fatalf("Accept-Language %s response = %d %q", supported.Tag, response.Code, response.Header().Get("Content-Language"))
			}
		})
	}
}
