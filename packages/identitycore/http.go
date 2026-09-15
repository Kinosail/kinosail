package identitycore

import "net/http"

// SecurityConfig connects shared HTTP security to app-specific request policy.
type SecurityConfig struct {
	MaxBody             int64
	Secure              func(*http.Request) bool
	UnsafeCrossOrigin   func(*http.Request) bool
	SessionCSRFRequired func(*http.Request) bool
	ValidCSRF           func(*http.Request) bool
	Reject              func(http.ResponseWriter, *http.Request, string, int)
}

// Security applies Player's response, request-size, origin, and CSRF policy.
func Security(next http.Handler, config SecurityConfig) http.Handler {
	if !validSecurity(next, config) {
		return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			http.Error(writer, "security middleware is unavailable", http.StatusInternalServerError)
		})
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		securityHeaders(writer.Header(), config.Secure(request))
		if request.Body != nil && request.Method != http.MethodGet && request.Method != http.MethodHead {
			request.Body = http.MaxBytesReader(writer, request.Body, config.MaxBody)
			if request.ContentLength > config.MaxBody {
				config.Reject(writer, request, "request is too large", http.StatusRequestEntityTooLarge)
				return
			}
		}
		if config.UnsafeCrossOrigin(request) || config.SessionCSRFRequired(request) && !config.ValidCSRF(request) {
			config.Reject(writer, request, "cross-origin request denied", http.StatusForbidden)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func securityHeaders(headers http.Header, secure bool) {
	headers.Set("Cache-Control", "no-store")
	headers.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; worker-src 'self' blob:; style-src 'self'; img-src 'self' data:; media-src 'self' blob:; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	headers.Set("Cross-Origin-Opener-Policy", "same-origin")
	headers.Set("Cross-Origin-Resource-Policy", "same-origin")
	headers.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	headers.Set("Referrer-Policy", "no-referrer")
	headers.Set("X-Content-Type-Options", "nosniff")
	headers.Set("X-Frame-Options", "DENY")
	headers.Set("X-XSS-Protection", "0")
	if secure {
		headers.Set("Strict-Transport-Security", "max-age=31536000")
	}
}

func validSecurity(next http.Handler, config SecurityConfig) bool {
	return next != nil && config.MaxBody > 0 && config.Secure != nil && config.UnsafeCrossOrigin != nil && config.SessionCSRFRequired != nil && config.ValidCSRF != nil && config.Reject != nil
}
