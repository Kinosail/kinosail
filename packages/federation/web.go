package federation

import "net/http"

// WebHooks keeps product sessions, local MFA verification, audit context, and error rendering app-owned.
type WebHooks[T any] struct {
	CurrentProfileID   func(*http.Request) string
	SecureRequest      func(*http.Request) bool
	ProfileID          func(T) string
	RequiresMFA        func(T) bool
	VerifySecondFactor func(string, string) bool
	SignIn             func(http.ResponseWriter, *http.Request, string) error
	Audit              func(*http.Request, T)
	Error              func(http.ResponseWriter, *http.Request, string, int)
	APIError           func(http.ResponseWriter, error, int)
}

func (hooks WebHooks[T]) signIn(writer http.ResponseWriter, request *http.Request, profile T) bool {
	if hooks.SignIn(writer, request, hooks.ProfileID(profile)) != nil {
		hooks.Error(writer, request, "SSO session could not be created", http.StatusInternalServerError)
		return false
	}
	hooks.Audit(request, profile)
	return true
}
