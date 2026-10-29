package federation

import (
	"errors"
	"net/http"
)

type WebHooks[T any] struct {
	CurrentProfileID   func(*http.Request) string
	LinkSession        func(*http.Request) string
	SecureRequest      func(*http.Request) bool
	ProfileID          func(T) string
	RequiresMFA        func(T) bool
	VerifySecondFactor func(string, string) bool
	SignIn             func(http.ResponseWriter, *http.Request, string) error
	Audit              func(*http.Request, T)
	Error              func(http.ResponseWriter, *http.Request, string, int)
	APIError           func(http.ResponseWriter, error, int)
	NotConfigured      func(*http.Request)
}

func (hooks WebHooks[T]) unavailable(writer http.ResponseWriter, request *http.Request, err error, message string, status int) {
	if errors.Is(err, ErrNotConfigured) && hooks.NotConfigured != nil {
		hooks.NotConfigured(request)
	}
	hooks.Error(writer, request, message, status)
}

func (hooks WebHooks[T]) linkSession(request *http.Request, profileID string) string {
	if profileID == "" || hooks.LinkSession == nil {
		return ""
	}
	return hooks.LinkSession(request)
}

func (hooks WebHooks[T]) signIn(writer http.ResponseWriter, request *http.Request, profile T) bool {
	if hooks.SignIn(writer, request, hooks.ProfileID(profile)) != nil {
		hooks.Error(writer, request, "SSO session could not be created", http.StatusInternalServerError)
		return false
	}
	hooks.Audit(request, profile)
	return true
}
