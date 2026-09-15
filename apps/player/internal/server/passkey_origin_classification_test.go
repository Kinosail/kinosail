package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/navigation"
)

func TestBrowserNavigationClassification(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		method  string
		path    string
		headers map[string]string
		want    bool
	}{
		{name: "document", method: http.MethodGet, path: "/login", headers: map[string]string{"Sec-Fetch-Dest": "document"}, want: true},
		{name: "navigation mode", method: http.MethodGet, path: "/login", headers: map[string]string{"Sec-Fetch-Mode": "navigate"}, want: true},
		{name: "HTML fallback", method: http.MethodGet, path: "/login", headers: map[string]string{"Accept": "text/html"}, want: true},
		{name: "HTML media type case", method: http.MethodGet, path: "/login", headers: map[string]string{"Accept": "Text/HTML; q=0.9"}, want: true},
		{name: "HTML minimum quality", method: http.MethodGet, path: "/login", headers: map[string]string{"Accept": "text/html;q=0.001"}, want: true},
		{name: "HTML rejected", method: http.MethodGet, path: "/login", headers: map[string]string{"Accept": "text/html;q=0"}},
		{name: "HTML quality out of range", method: http.MethodGet, path: "/login", headers: map[string]string{"Accept": "text/html;q=2"}},
		{name: "HTML quality malformed", method: http.MethodGet, path: "/login", headers: map[string]string{"Accept": "text/html;q=Inf"}},
		{name: "HTML substring", method: http.MethodGet, path: "/login", headers: map[string]string{"Accept": "application/not-text/html"}},
		{name: "oversized Accept", method: http.MethodGet, path: "/login", headers: map[string]string{"Accept": strings.Repeat("a", 4097)}},
		{name: "too many media types", method: http.MethodGet, path: "/login", headers: map[string]string{"Accept": strings.Repeat("text/plain,", 32) + "text/html"}},
		{name: "API HTML", method: http.MethodGet, path: "/api/v1/library", headers: map[string]string{"Accept": "text/html"}},
		{name: "Jellyfin HTML", method: http.MethodGet, path: "/Users/Me", headers: map[string]string{"Accept": "text/html"}},
		{name: "stylesheet", method: http.MethodGet, path: "/static/app.css", headers: map[string]string{"Sec-Fetch-Dest": "style", "Accept": "text/css,*/*;q=0.1"}},
		{name: "media", method: http.MethodGet, path: "/stream/test", headers: map[string]string{"Sec-Fetch-Dest": "video", "Accept": "video/*"}},
		{name: "HTMX", method: http.MethodGet, path: "/", headers: map[string]string{"HX-Request": "true", "Accept": "*/*"}},
		{name: "non-navigation mutation", method: http.MethodPut, path: "/settings", headers: map[string]string{"Accept": "text/html"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(t.Context(), test.method, test.path, nil)
			for name, value := range test.headers {
				request.Header.Set(name, value)
			}
			if got := navigation.IsBrowserRequest(request, authenticationErrorRequest(request)); got != test.want {
				t.Fatalf("IsBrowserRequest() = %t, want %t", got, test.want)
			}
		})
	}
}
