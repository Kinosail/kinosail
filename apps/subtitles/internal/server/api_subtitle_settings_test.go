package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestSubtitleLanguageAPIRejectsInvalidInputWithoutChangingSettings(t *testing.T) {
	handler, token := apiServer(t)
	path := "/api/v1/settings/subtitles"
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPut, path, map[string]any{"languages": []string{"en", "fr"}}), http.StatusOK, `"status":"saved"`)
	want := []string{"en", "fr"}
	tooMany := []string{"en", "fr", "de", "es", "it", "nl", "pl", "pt", "ru", "uk", "tr", "ar", "fa", "he", "hi", "bn", "ur", "id", "ms", "vi", "th"}
	cases := []struct {
		name string
		body any
	}{
		{"missing", map[string]any{}},
		{"unknown", map[string]any{"languages": []string{"en", "not-a-language"}}},
		{"provider spellings", map[string]any{"languages": []string{"ea", "sp", "at", "pm", "zh_bg"}}},
		{"oversized value", map[string]any{"languages": []string{"en", strings.Repeat("x", 128)}}},
		{"oversized list", map[string]any{"languages": tooMany}},
		{"canonical duplicate", map[string]any{"languages": []string{"pt-br", "pt-BR"}}},
		{"conflicting fields", map[string]any{"language": "en", "languages": []string{"fr"}}},
		{"unknown field", map[string]any{"languages": []string{"en"}, "fallback": true}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			response := apiCall(t, handler, token, http.MethodPut, path, test.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
			}
			if got := subtitleLanguagesFromAPI(t, handler, token); !reflect.DeepEqual(got, want) {
				t.Fatalf("rejected input changed languages: got %q, want %q", got, want)
			}
		})
	}
}

func TestSubtitleLanguageAPIRejectsMalformedJSONWithoutChangingSettings(t *testing.T) {
	handler, token := apiServer(t)
	path := "/api/v1/settings/subtitles"
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPut, path, map[string]any{"languages": []string{"en", "fr"}}), http.StatusOK, `"status":"saved"`)
	for _, body := range []string{
		`{"languages":"en"}`,
		`{"languages":["en",42]}`,
		`{"language":"en","Language":"fr"}`,
		strings.Repeat(" ", 4097),
		`{"languages":["en"]} {"languages":["fr"]}`,
		`{"languages":["en"]`,
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, path, bytes.NewBufferString(body))
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Errorf("%q status = %d, body = %q", body, response.Code, response.Body.String())
		}
		if got := subtitleLanguagesFromAPI(t, handler, token); !reflect.DeepEqual(got, []string{"en", "fr"}) {
			t.Fatalf("malformed input changed languages: got %q", got)
		}
	}
}

func TestSubtitleLanguageAPISingularUpdatePreservesSecondaryPreferences(t *testing.T) {
	handler, token := apiServer(t)
	path := "/api/v1/settings/subtitles"
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPut, path, map[string]any{"languages": []string{"en", "es", "fr"}}), http.StatusOK, `"status":"saved"`)
	for _, update := range []struct {
		language string
		want     []string
	}{{"fr", []string{"fr", "en", "es"}}, {"de", []string{"de", "en", "es"}}, {"it", []string{"it", "en", "es"}}} {
		assertAPIBody(t, apiCall(t, handler, token, http.MethodPut, path, map[string]any{"language": update.language}), http.StatusOK, `"status":"saved"`)
		if got := subtitleLanguagesFromAPI(t, handler, token); !reflect.DeepEqual(got, update.want) {
			t.Fatalf("language %q saved %q; want %q", update.language, got, update.want)
		}
	}
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPut, path, map[string]any{"languages": []string{"en", "pt-BR", "es"}}), http.StatusOK, `"status":"saved"`)
	response := apiCall(t, handler, token, http.MethodPut, path, map[string]any{"language": "pt"})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("overlapping singular update = %d, body = %q", response.Code, response.Body.String())
	}
	if got := subtitleLanguagesFromAPI(t, handler, token); !reflect.DeepEqual(got, []string{"en", "pt-BR", "es"}) {
		t.Fatalf("rejected singular update changed languages: %q", got)
	}
}

func subtitleLanguagesFromAPI(t *testing.T, handler http.Handler, token string) []string {
	t.Helper()
	response := apiCall(t, handler, token, http.MethodGet, "/api/v1/settings", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("settings status = %d, body = %q", response.Code, response.Body.String())
	}
	var settings struct {
		SubtitleLanguages []string `json:"subtitleLanguages"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	return settings.SubtitleLanguages
}
