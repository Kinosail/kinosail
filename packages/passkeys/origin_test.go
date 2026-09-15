package passkeys

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOriginValidationAndMatching(t *testing.T) { //nolint:cyclop // One table verifies the complete origin trust boundary.
	origin, err := ParseOrigin("https://Media.Example:38127/")
	if err != nil || origin.RPID() != "Media.Example" || origin.String() != "https://Media.Example:38127" || !origin.RedirectPages() {
		t.Fatalf("origin = %#v, %v", origin, err)
	}
	for _, raw := range []string{"", "ftp://example.test", "https://user@example.test", "https://example.test/path", "https://example.test?x=1", "https://example.test#fragment", "https://", "://", strings.Repeat("x", maxOriginLength+1)} {
		if _, parseErr := ParseOrigin(raw); !errors.Is(parseErr, ErrInvalidOrigin) {
			t.Fatalf("invalid origin %q = %v", raw, parseErr)
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://media.example:38127/account", nil)
	insecureRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://media.example:38127/account", nil)
	if !origin.Matches(request, true) || origin.Matches(insecureRequest, false) || origin.Matches(nil, true) {
		t.Fatal("origin request match is incorrect")
	}
	localhost, _ := ParseOrigin("http://localhost:38127")
	loopback, _ := ParseOrigin("https://127.0.0.1:38127")
	if localhost.RedirectPages() || loopback.RedirectPages() {
		t.Fatal("local origins require a page redirect")
	}
}

func TestOriginPageRedirectBoundsMethodPathAndQuery(t *testing.T) {
	origin, _ := ParseOrigin("https://media.example")
	for _, test := range []struct {
		method, target, want string
		status               int
	}{
		{http.MethodGet, "/account?passkey=offer", "https://media.example/account?passkey=offer", http.StatusTemporaryRedirect},
		{http.MethodHead, "/login", "https://media.example/login", http.StatusTemporaryRedirect},
		{http.MethodPost, "/setup?step=owner", "https://media.example/setup?step=owner", http.StatusSeeOther},
		{http.MethodPost, "/private?secret=1", "https://media.example/", http.StatusSeeOther},
	} {
		request := httptest.NewRequestWithContext(t.Context(), test.method, test.target, nil)
		got, status := origin.PageRedirect(request)
		if got != test.want || status != test.status {
			t.Fatalf("%s %s = %q, %d", test.method, test.target, got, status)
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.URL.Path = "//outside"
	target, _ := origin.PageRedirect(request)
	if target != "https://media.example/" {
		t.Fatalf("unsafe path = %q", target)
	}
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?"+strings.Repeat("q", 4097), nil)
	target, _ = origin.PageRedirect(request)
	if target != "https://media.example/" {
		t.Fatalf("oversized query = %q", target)
	}
}

func TestEngineCanonicalPageOrigin(t *testing.T) {
	origin, _ := ParseOrigin("https://media.example")
	engine := &Engine{origin: origin}
	matching := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://media.example/account", nil)
	mismatch := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://alias.example/account", nil)
	if !engine.MatchesRequest(matching, true) || (*Engine)(nil).MatchesRequest(matching, true) {
		t.Fatal("engine origin matching is incorrect")
	}
	for _, test := range []struct {
		name    string
		engine  *Engine
		request *http.Request
		enabled bool
		want    bool
	}{
		{"disabled", engine, mismatch, false, true},
		{"missing engine", nil, mismatch, true, true},
		{"matching", engine, matching, true, true},
		{"redirect", engine, mismatch, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			if got := test.engine.RequirePageOrigin(response, test.request, test.enabled, true); got != test.want {
				t.Fatalf("RequirePageOrigin() = %v, want %v", got, test.want)
			}
			if !test.want && (response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != "https://media.example/account") {
				t.Fatalf("redirect = %d %q", response.Code, response.Header().Get("Location"))
			}
		})
	}
}
