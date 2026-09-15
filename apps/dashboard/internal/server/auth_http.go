package server

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/MikeO7/kinosail-dashboard/internal/auth"
)

func registerAuthentication(mux *http.ServeMux, config Config, authentication *auth.Manager) {
	limiter := newLoginLimiter()
	mux.HandleFunc("POST /api/v1/setup", setupHandler(config, authentication, limiter))
	mux.HandleFunc("POST /api/v1/session", loginHandler(config, authentication, limiter))
	mux.HandleFunc("DELETE /api/v1/session", logoutHandler(config, authentication))
	mux.HandleFunc("POST /api/v1/owner/password", passwordHandler(authentication))
	mux.HandleFunc("POST /api/v1/mcp-token", issueMCPTokenHandler(authentication))
	mux.HandleFunc("DELETE /api/v1/mcp-token", revokeMCPTokenHandler(authentication))
}

func setupHandler(config Config, authentication *auth.Manager, limiter *loginLimiter) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct {
			Name     string `json:"name"`
			Password string `json:"password"`
			Device   string `json:"device"`
		}
		if !readJSON(writer, request, &input, 16<<10) {
			return
		}
		allowed, retryAfter := limiter.allow(request, "setup")
		if !allowed {
			writer.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			apiError(writer, errors.New("too many setup attempts"), http.StatusTooManyRequests)
			return
		}
		session, err := authentication.Setup(request.Context(), input.Name, input.Password, input.Device)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, auth.ErrAlreadyConfigured) {
				status = http.StatusConflict
			}
			if errors.Is(err, auth.ErrState) {
				status = http.StatusInternalServerError
				err = auth.ErrState
			}
			apiError(writer, err, status)
			return
		}
		limiter.success(request, "setup")
		setSessionCookie(writer, session, config.SecureCookies)
		writer.Header().Set("X-Kinosail-Login-Next", "/?passkey=offer")
		writeJSON(writer, sessionResponse(session), http.StatusCreated)
	}
}

func loginHandler(config Config, authentication *auth.Manager, limiter *loginLimiter) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct {
			Name     string `json:"name"`
			Password string `json:"password"`
			Device   string `json:"device"`
		}
		if !readJSON(writer, request, &input, 16<<10) {
			return
		}
		allowed, retryAfter := limiter.allow(request, input.Name)
		if !allowed {
			writer.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			apiError(writer, errors.New("too many login attempts"), http.StatusTooManyRequests)
			return
		}
		session, err := authentication.Login(request.Context(), input.Name, input.Password, input.Device)
		if err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, auth.ErrState) {
				status = http.StatusInternalServerError
				err = auth.ErrState
			}
			apiError(writer, err, status)
			return
		}
		limiter.success(request, input.Name)
		setSessionCookie(writer, session, config.SecureCookies)
		writer.Header().Set("X-Kinosail-Login-Next", "/?passkey=offer")
		writeJSON(writer, sessionResponse(session), http.StatusCreated)
	}
}

func logoutHandler(config Config, authentication *auth.Manager) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		identity, found := requireOwner(writer, request)
		if !found || !requireCSRF(writer, request, identity) {
			return
		}
		if err := authentication.Logout(request.Context(), request); err != nil {
			apiError(writer, errors.New("could not end session"), http.StatusInternalServerError)
			return
		}
		http.SetCookie(writer, &http.Cookie{Name: auth.SessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: config.SecureCookies}) // #nosec G124 -- Secure follows the validated public URL scheme; local HTTP remains supported.
		writer.WriteHeader(http.StatusNoContent)
	}
}

func passwordHandler(authentication *auth.Manager) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		identity, found := requireOwner(writer, request)
		if !found || !requireCSRF(writer, request, identity) {
			return
		}
		if identity.ViaBearer {
			apiError(writer, errors.New("password changes require a browser session"), http.StatusForbidden)
			return
		}
		var input struct {
			CurrentPassword string `json:"currentPassword"`
			NewPassword     string `json:"newPassword"`
			ConfirmPassword string `json:"confirmPassword"`
		}
		if !readJSON(writer, request, &input, 16<<10) {
			return
		}
		cookie, err := request.Cookie(auth.SessionCookie)
		if err != nil {
			apiError(writer, errors.New("authentication required"), http.StatusUnauthorized)
			return
		}
		err = authentication.ChangePassword(request.Context(), input.CurrentPassword, input.NewPassword, input.ConfirmPassword, cookie.Value)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, auth.ErrState) {
				status, err = http.StatusInternalServerError, auth.ErrState
			}
			apiError(writer, err, status)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

func issueMCPTokenHandler(authentication *auth.Manager) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		identity, found := requireOwner(writer, request)
		if !found || !requireCSRF(writer, request, identity) {
			return
		}
		var input struct{}
		if !readJSON(writer, request, &input, 1024) {
			return
		}
		session, err := authentication.IssueMCPToken(request.Context())
		if err != nil {
			apiError(writer, auth.ErrState, http.StatusInternalServerError)
			return
		}
		writeJSON(writer, map[string]any{"token": session.Token, "expiresAt": session.ExpiresAt}, http.StatusCreated)
	}
}

func revokeMCPTokenHandler(authentication *auth.Manager) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		identity, found := requireOwner(writer, request)
		if !found || !requireCSRF(writer, request, identity) {
			return
		}
		var input struct{}
		if !readJSON(writer, request, &input, 1024) {
			return
		}
		if err := authentication.RevokeMCPToken(request.Context()); err != nil {
			apiError(writer, auth.ErrState, http.StatusInternalServerError)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

func setSessionCookie(writer http.ResponseWriter, session auth.Session, secure bool) {
	http.SetCookie(writer, &http.Cookie{Name: auth.SessionCookie, Value: session.Token, Path: "/", Expires: session.ExpiresAt, MaxAge: int(time.Until(session.ExpiresAt).Seconds()), HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: secure}) // #nosec G124 -- Secure follows the validated public URL scheme; local HTTP remains supported.
}

func sessionResponse(session auth.Session) map[string]any {
	return map[string]any{"owner": map[string]string{"id": session.ID, "name": session.Name}, "csrf": session.CSRF, "expiresAt": session.ExpiresAt}
}
