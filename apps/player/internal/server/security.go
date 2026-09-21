package server

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/serverdiscovery"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

// Remote marks every request from a dedicated public listener without trusting caller headers.
func Remote(next http.Handler) http.Handler {
	return identitycore.Remote(next)
}

func publicInternetRequest(request *http.Request) bool {
	return identitycore.RemoteRequest(request)
}

const maxRequestBody = 1 << 20

func hardenedHTTPClient(timeout time.Duration) *http.Client {
	return outboundHTTPClient(timeout, allowedOutboundIP)
}

func localIntegrationHTTPClient(timeout time.Duration) *http.Client {
	return outboundHTTPClient(timeout, allowedIntegrationIP)
}

func outboundHTTPClient(timeout time.Duration, allowed func(net.IP) bool) *http.Client {
	return identitycore.OutboundHTTPClient(timeout, allowed)
}

func allowedOutboundIP(address net.IP) bool {
	return identitycore.AllowedOutboundIP(address)
}

func allowedIntegrationIP(address net.IP) bool {
	return identitycore.AllowedIntegrationIP(address)
}

func trustedProxy(token string, next http.Handler) http.Handler {
	return identitycore.TrustedProxy(token, next)
}

func allowedHost(rawURL string, aliases []string, next http.Handler) http.Handler {
	aliases = append(append([]string(nil), aliases...), serverdiscovery.LocalAliases(rawURL)...)
	return identitycore.AllowedHost(rawURL, aliases, "38127", next, localizedError)
}

func security(next http.Handler) http.Handler {
	return trustedhttps.LimitSettingsForms(identitycore.Security(next, identitycore.SecurityConfig{MaxBody: maxRequestBody, Secure: secureRequest, UnsafeCrossOrigin: unsafeCrossOrigin, SessionCSRFRequired: sessionCSRFRequired, ValidCSRF: httpguard.ValidCSRF, Reject: localizedError}))
}

func sessionCSRFRequired(request *http.Request) bool {
	if request.Method == http.MethodGet || request.Method == http.MethodHead || request.Method == http.MethodOptions || request.UserAgent() == "" || !browserSessionCookie(request) {
		return false
	}
	switch request.URL.Path {
	case "/api/v1/passkeys/login/begin", "/api/v1/passkeys/login/finish", "/auth/passkeys/login/begin", "/auth/passkeys/login/finish":
		return request.Method != http.MethodPost
	}
	return true
}

func secureRequest(request *http.Request) bool {
	return request.TLS != nil || strings.EqualFold(strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-Proto"), ",")[0]), "https")
}

func unsafeCrossOrigin(request *http.Request) bool { //nolint:cyclop // Browser request evidence is checked explicitly at this security boundary.
	if request.Method == http.MethodGet || request.Method == http.MethodHead || request.Method == http.MethodOptions {
		return false
	}
	rawOrigin := request.Header.Get("Origin")
	if rawOrigin != "" && rawOrigin != "null" {
		origin, err := url.Parse(rawOrigin)
		return err != nil || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || !sameRequestOrigin(request, origin)
	}
	if strings.EqualFold(request.Header.Get("Sec-Fetch-Site"), "cross-site") {
		return true
	}
	if rawOrigin == "null" {
		return !strings.EqualFold(request.Header.Get("Sec-Fetch-Site"), "same-origin")
	}
	return browserSessionCookie(request) && request.UserAgent() != "" && !strings.EqualFold(request.Header.Get("Sec-Fetch-Site"), "same-origin")
}

func browserSessionCookie(request *http.Request) bool {
	cookie, _ := request.Cookie("__Host-kinosail_session")
	return cookie != nil
}

func sameRequestOrigin(request *http.Request, origin *url.URL) bool {
	scheme := "http"
	if secureRequest(request) {
		scheme = "https"
	}
	if !strings.EqualFold(origin.Scheme, scheme) {
		return false
	}
	return strings.EqualFold(canonicalOriginHost(origin.Host, scheme), canonicalOriginHost(request.Host, scheme))
}

func canonicalOriginHost(host, scheme string) string {
	name, port, err := net.SplitHostPort(host)
	if err != nil {
		name, port = host, map[bool]string{true: "443", false: "80"}[scheme == "https"]
	}
	return strings.ToLower(net.JoinHostPort(strings.TrimSuffix(name, "."), port))
}

func protectApplicationTransport(auth *authentication, config Config, pattern func(*http.Request) string, handler http.Handler) http.Handler {
	return trustedProxy(config.ProxyToken, allowedHost(config.AuthURL, config.Configuration.Strings("tls.hosts"), observeRequests(auth.audit, pattern, tripwirePublic(auth.audit, publicRequestLimits(handler)))))
}
