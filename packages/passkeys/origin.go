package passkeys

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const maxOriginLength = 2048

var ErrInvalidOrigin = errors.New("passkey URL must be an HTTP origin")

// Origin is one validated WebAuthn relying-party origin.
type Origin struct {
	rpID string
	url  string
}

// ParseOrigin validates one exact HTTP or HTTPS origin.
func ParseOrigin(rawURL string) (Origin, error) {
	if len(rawURL) > maxOriginLength {
		return Origin{}, ErrInvalidOrigin
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || !validOrigin(parsed) {
		return Origin{}, ErrInvalidOrigin
	}
	return Origin{rpID: parsed.Hostname(), url: parsed.Scheme + "://" + parsed.Host}, nil
}

// RPID returns the WebAuthn relying-party identifier.
func (origin Origin) RPID() string { return origin.rpID }

// String returns the normalized scheme and authority.
func (origin Origin) String() string { return origin.url }

// Matches reports whether one request uses this origin.
func (origin Origin) Matches(request *http.Request, secure bool) bool {
	if request == nil {
		return false
	}
	scheme := "http"
	if secure || request.TLS != nil {
		scheme = "https"
	}
	return strings.EqualFold(scheme+"://"+request.Host, origin.url)
}

// RedirectPages reports whether noncanonical browser pages need a redirect.
func (origin Origin) RedirectPages() bool {
	ip := net.ParseIP(origin.rpID)
	return !strings.EqualFold(origin.rpID, "localhost") && (ip == nil || !ip.IsLoopback())
}

// PageRedirect returns the safe canonical target for one browser request.
func (origin Origin) PageRedirect(request *http.Request) (string, int) {
	path := safePagePath(request.URL.EscapedPath())
	status := http.StatusTemporaryRedirect
	query := request.URL.RawQuery
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		status = http.StatusSeeOther
		if path != "/setup" && path != "/login" {
			path, query = "/", ""
		}
	}
	target := origin.url + path
	if query != "" && len(path)+1+len(query) <= 4096 {
		target += "?" + query
	}
	return target, status
}

func safePagePath(path string) string {
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || len(path) > maxOriginLength {
		return "/"
	}
	return path
}

func validOrigin(parsed *url.URL) bool {
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && (parsed.Path == "" || parsed.Path == "/")
}
