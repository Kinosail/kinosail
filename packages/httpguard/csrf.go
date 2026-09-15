package httpguard

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
)

// CSRFForRequest derives a token only when the request has a session cookie.
func CSRFForRequest(request *http.Request) string {
	if request == nil {
		return ""
	}
	if cookie, _ := request.Cookie("__Host-kinosail_session"); cookie != nil && cookie.Value != "" {
		return CSRFToken(cookie.Value)
	}
	return ""
}

// CSRFToken returns Player's domain-separated, HTML-safe session token.
func CSRFToken(session string) string {
	digest := sha256.Sum256([]byte("kinosail-csrf\x00" + session))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

// ValidCSRF validates one header or form token and consumes form token fields.
func ValidCSRF(request *http.Request) bool {
	expected := CSRFForRequest(request)
	if expected == "" {
		return true
	}
	values := request.Header.Values("X-Kinosail-CSRF")
	if len(values) == 0 {
		if err := request.ParseForm(); err != nil {
			return false
		}
		values = request.PostForm["_csrf"]
		request.PostForm.Del("_csrf")
		request.Form.Del("_csrf")
	}
	return len(values) == 1 && subtle.ConstantTimeCompare([]byte(values[0]), []byte(expected)) == 1
}
