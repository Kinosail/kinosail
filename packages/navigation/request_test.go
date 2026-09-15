package navigation

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrowserRequestClassification(t *testing.T) { //nolint:cyclop // The table covers each independent request classification boundary.
	t.Parallel()
	for _, test := range []struct {
		name    string
		method  string
		headers map[string]string
		authErr bool
		want    bool
	}{
		{name: "document", method: http.MethodGet, headers: map[string]string{"Sec-Fetch-Dest": "document"}, want: true},
		{name: "navigation mode", method: http.MethodGet, headers: map[string]string{"Sec-Fetch-Mode": "navigate"}, want: true},
		{name: "HTML fallback", method: http.MethodGet, headers: map[string]string{"Accept": "text/html"}, want: true},
		{name: "HTML full quality", method: http.MethodGet, headers: map[string]string{"Accept": "text/html;q=1"}, want: true},
		{name: "HTML media type case", method: http.MethodHead, headers: map[string]string{"Accept": "Text/HTML; q=0.9"}, want: true},
		{name: "HTML minimum quality", method: http.MethodPost, headers: map[string]string{"Accept": "text/html;q=0.001"}, want: true},
		{name: "HTML maximum quality", method: http.MethodGet, headers: map[string]string{"Accept": "text/html;q=1.000"}, want: true},
		{name: "HTML rejected", method: http.MethodGet, headers: map[string]string{"Accept": "text/html;q=0"}},
		{name: "HTML zero fraction", method: http.MethodGet, headers: map[string]string{"Accept": "text/html;q=0.000"}},
		{name: "HTML quality out of range", method: http.MethodGet, headers: map[string]string{"Accept": "text/html;q=2"}},
		{name: "HTML quality malformed", method: http.MethodGet, headers: map[string]string{"Accept": "text/html;q=Inf"}},
		{name: "HTML quality too precise", method: http.MethodGet, headers: map[string]string{"Accept": "text/html;q=0.0001"}},
		{name: "HTML quality invalid digit", method: http.MethodGet, headers: map[string]string{"Accept": "text/html;q=0.a"}},
		{name: "HTML quality above one", method: http.MethodGet, headers: map[string]string{"Accept": "text/html;q=1.001"}},
		{name: "HTML substring", method: http.MethodGet, headers: map[string]string{"Accept": "application/not-text/html"}},
		{name: "malformed media type then HTML", method: http.MethodGet, headers: map[string]string{"Accept": "bad type,text/html"}, want: true},
		{name: "oversized Accept", method: http.MethodGet, headers: map[string]string{"Accept": strings.Repeat("a", 4097)}},
		{name: "too many media types", method: http.MethodGet, headers: map[string]string{"Accept": strings.Repeat("text/plain,", 32) + "text/html"}},
		{name: "authentication error", method: http.MethodGet, headers: map[string]string{"Accept": "text/html"}, authErr: true},
		{name: "stylesheet", method: http.MethodGet, headers: map[string]string{"Sec-Fetch-Dest": "style", "Accept": "text/css,*/*;q=0.1"}},
		{name: "media", method: http.MethodGet, headers: map[string]string{"Sec-Fetch-Dest": "video", "Accept": "video/*"}},
		{name: "HTMX", method: http.MethodGet, headers: map[string]string{"HX-Request": "true", "Accept": "*/*"}},
		{name: "non-navigation mutation", method: http.MethodPut, headers: map[string]string{"Accept": "text/html"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(t.Context(), test.method, "/login", nil)
			for name, value := range test.headers {
				request.Header.Set(name, value)
			}
			if got := IsBrowserRequest(request, test.authErr); got != test.want {
				t.Fatalf("IsBrowserRequest() = %t, want %t", got, test.want)
			}
		})
	}
	if IsBrowserRequest(nil, false) {
		t.Fatal("nil request is a browser navigation")
	}
}
