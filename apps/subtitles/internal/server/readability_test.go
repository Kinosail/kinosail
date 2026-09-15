package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSharedTextStatesKeepReadableContrast(t *testing.T) {
	t.Parallel()

	handler := server.New(server.Config{})
	styles := httptest.NewRecorder()
	handler.ServeHTTP(styles, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))
	css := styles.Body.String()
	for _, expected := range []string{
		`input::placeholder{color:var(--muted)}`,
		`.settings-flow fieldset[disabled],.settings-shell>section fieldset[disabled]{opacity:.72}`,
		`.settings-flow fieldset[disabled] button:disabled,.settings-shell>section fieldset[disabled] button:disabled{opacity:1}`,
		`button:disabled{opacity:1;background:var(--surface-3);color:var(--muted)}[aria-disabled=true]{opacity:.62}`,
	} {
		if !strings.Contains(css, expected) {
			t.Fatalf("readability rule missing %q", expected)
		}
	}
}
