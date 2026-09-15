package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestShowCastTemplateUsesPublicPhotosAndEscapesMetadata(t *testing.T) {
	for _, populated := range []bool{false, true} {
		data := showPageData{Show: library.Show{Title: "Example"}}
		if populated {
			data.People = []catalogapi.ClientPerson{{Name: "Actor <script>", Role: "Lead & friend", Image: "/person/episode/0?scope=show"}, {Name: "Voice"}}
		}
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/example", nil)
		if err := showView.Execute(response, request, data); err != nil {
			t.Fatal(err)
		}
		body := response.Body.String()
		if strings.Contains(body, `class="cast-section"`) != populated {
			t.Fatalf("cast visibility mismatch: %s", body)
		}
		if populated {
			assertShowCastEscaping(t, body)
		}
	}
}

func assertShowCastEscaping(t *testing.T, body string) {
	t.Helper()
	for _, expected := range []string{`src="/person/episode/0?scope=show"`, "Actor &lt;script&gt;", "Lead &amp; friend", "Voice"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q", expected)
		}
	}
}
