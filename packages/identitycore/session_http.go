package identitycore

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	compatibilityPathSegment = regexp.MustCompile(`^[A-Za-z][A-Za-z-]{0,31}$`)
	contentRangePattern      = regexp.MustCompile(`^bytes (?:(\d+)-(\d+)|\*)/(\d+)$`)
)

// SafeCompatibilityPath normalizes an otherwise-unmatched compatibility route for logs.
func SafeCompatibilityPath(request *http.Request, pattern string) string {
	if pattern != "" || len(request.URL.Path) > 512 || MediaBrowserValue(request, "Client") == "" {
		return ""
	}
	segments := strings.Split(strings.TrimPrefix(request.URL.Path, "/"), "/")
	if len(segments) == 0 || len(segments) > 12 {
		return ""
	}
	for index, segment := range segments {
		if !compatibilityPathSegment.MatchString(segment) {
			segments[index] = "{value}"
		}
	}
	return "/" + strings.Join(segments, "/")
}

// ValidContentRange accepts one internally consistent bounded response range.
func ValidContentRange(value string) bool {
	match := contentRangePattern.FindStringSubmatch(value)
	if len(value) > 128 || match == nil {
		return false
	}
	if match[1] == "" {
		return true
	}
	start, _ := strconv.ParseUint(match[1], 10, 64)
	end, _ := strconv.ParseUint(match[2], 10, 64)
	size, _ := strconv.ParseUint(match[3], 10, 64)
	return start <= end && end < size
}

type QuerySessionToken func(*http.Request) (string, string)

func SessionToken(request *http.Request, query QuerySessionToken) string {
	token, _ := sessionToken(request, query)
	return token
}

func SessionTokenSource(request *http.Request, query QuerySessionToken) string {
	_, source := sessionToken(request, query)
	return source
}

func sessionToken(request *http.Request, query QuerySessionToken) (string, string) { //nolint:cyclop // Compatibility token sources use a fixed precedence.
	if request == nil {
		return "", "none"
	}
	if authorization := request.Header.Get("Authorization"); strings.HasPrefix(authorization, "Bearer ") {
		return strings.TrimPrefix(authorization, "Bearer "), "authorization-bearer"
	}
	for _, header := range []struct{ name, source string }{{"X-Emby-Token", "x-emby-token"}, {"X-MediaBrowser-Token", "x-mediabrowser-token"}, {"ApiKey", "apikey-header"}} {
		if value := request.Header.Get(header.name); value != "" {
			return value, header.source
		}
	}
	if token := MediaBrowserValue(request, "Token"); token != "" {
		return token, "media-browser-authorization"
	}
	if query != nil {
		if token, source := query(request); token != "" {
			return token, source
		}
	}
	if cookie, _ := request.Cookie("__Host-kinosail_session"); cookie != nil {
		return cookie.Value, "kinosail-session-cookie"
	}
	return "", "none"
}

func MediaBrowserValue(request *http.Request, name string) string {
	if request == nil {
		return ""
	}
	for _, authorization := range []string{request.Header.Get("Authorization"), request.Header.Get("X-Emby-Authorization")} {
		for _, part := range strings.Split(authorization, ",") {
			key, value, found := strings.Cut(strings.TrimSpace(part), "=")
			if scheme, parameter, separated := strings.Cut(key, " "); separated {
				if strings.EqualFold(scheme, "MediaBrowser") {
					key = parameter
				}
			}
			if found && strings.EqualFold(key, name) {
				return strings.Trim(strings.TrimSpace(value), `"`)
			}
		}
	}
	return ""
}

func SessionKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func NormalizeSessions(sessions map[string]Session, now func() time.Time) {
	if now == nil {
		now = time.Now
	}
	created := now().Unix()
	for key, session := range sessions {
		normalized := SessionKeyIfNeeded(key)
		if normalized != key {
			delete(sessions, key)
		}
		if session.Name == "" {
			session.Name = "Legacy device"
		}
		if session.CreatedAt == 0 {
			session.CreatedAt = created
		}
		sessions[normalized] = session
	}
}

func SessionKeyIfNeeded(key string) string {
	if _, err := hex.DecodeString(key); len(key) == 64 && err == nil {
		return key
	}
	return SessionKey(key)
}

func CleanDeviceName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Web browser"
	}
	for _, browser := range []struct{ marker, name string }{{"Firefox/", "Firefox"}, {"Edg/", "Microsoft Edge"}, {"Chrome/", "Chrome"}, {"Safari/", "Safari"}} {
		if strings.Contains(name, browser.marker) {
			return browser.name
		}
	}
	return name[:min(len(name), 80)]
}

func SessionCookie(token string) *http.Cookie {
	return &http.Cookie{Name: "__Host-kinosail_session", Value: token, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode}
}

func PublicSessionCookie(token string, now time.Time) *http.Cookie {
	return &http.Cookie{Name: "__Host-kinosail_session", Value: token, Path: "/", MaxAge: 8 * 60 * 60, Expires: now.Add(8 * time.Hour), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode}
}
