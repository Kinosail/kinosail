package identitycore

import (
	"errors"
	"net/http"
	"time"
)

// DenialConfig connects shared access denials to app presentation and auditing.
type DenialConfig struct {
	Denied              func(*http.Request, string)
	NotFound            func(http.ResponseWriter, *http.Request)
	AuthenticationError func(*http.Request) bool
	Error               func(http.ResponseWriter, *http.Request, string, int)
	JSON                func(http.ResponseWriter, any, int)
}

// RespondDenial renders and audits one access denial.
func RespondDenial(writer http.ResponseWriter, request *http.Request, denial Denial, config DenialConfig) {
	if !validDenialConfig(config) || denial == Allowed {
		http.Error(writer, "access denial is unavailable", http.StatusInternalServerError)
		return
	}
	config.Denied(request, denial.Reason())
	if denial == RemoteRouteDenied {
		config.NotFound(writer, request)
		return
	}
	if denial == MFAEnrollmentRequired && !config.AuthenticationError(request) {
		http.Redirect(writer, request, "/account?mfa=required", http.StatusSeeOther)
		return
	}
	if denial == MFAEnrollmentRequired {
		config.JSON(writer, map[string]any{"error": denial.Message(), "mfaEnrollmentRequired": true}, denial.Status())
		return
	}
	config.Error(writer, request, denial.Message(), denial.Status())
}

func validDenialConfig(config DenialConfig) bool {
	return config.Denied != nil && config.NotFound != nil && config.AuthenticationError != nil && config.Error != nil && config.JSON != nil
}

// OwnerConfig connects shared Owner authorization to app identity and presentation.
type OwnerConfig struct {
	Identity              func(*http.Request) (owner, local bool)
	Managed               func(*http.Request) bool
	RecentlyAuthenticated func(*http.Request, time.Duration) bool
	AuthenticationError   func(*http.Request) bool
	StepUpPath            func(*http.Request) string
	Error                 func(http.ResponseWriter, *http.Request, string, int)
	JSON                  func(http.ResponseWriter, any, int)
}

// Owner applies Player's Owner and recent-authentication middleware.
func Owner(next http.Handler, config OwnerConfig) http.Handler {
	if next == nil || !validOwnerConfig(config) {
		return unavailableHandler()
	}
	return ownerHandler(next, config, false)
}

// OwnerSensitive requires fresh authentication even for a read that exports private state.
func OwnerSensitive(next http.Handler, config OwnerConfig) http.Handler {
	if next == nil || !validOwnerConfig(config) {
		return unavailableHandler()
	}
	return ownerHandler(next, config, true)
}

func ownerHandler(next http.Handler, config OwnerConfig, sensitive bool) http.HandlerFunc {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		owner, local := config.Identity(request)
		managed := config.Managed(request)
		if RemoteRequest(request) {
			config.Error(writer, request, "Owner access is private", http.StatusForbidden)
			return
		}
		safe := !sensitive && (request.Method == http.MethodGet || request.Method == http.MethodHead)
		recent := local || managed || safe || config.RecentlyAuthenticated(request, 10*time.Minute)
		switch EvaluateOwner(owner, local, managed, safe, recent) { //nolint:exhaustive // EvaluateOwner returns only the three handled Owner denials.
		case Allowed:
			next.ServeHTTP(writer, request)
		case OwnerStepUpRequired:
			if config.AuthenticationError(request) {
				config.JSON(writer, map[string]any{"error": "recent authentication required", "stepUpRequired": true}, http.StatusForbidden)
			} else {
				http.Redirect(writer, request, config.StepUpPath(request), http.StatusSeeOther)
			}
		case OwnerRequired:
			config.Error(writer, request, "Owner access required", http.StatusForbidden)
		}
	})
}

func validOwnerConfig(config OwnerConfig) bool {
	return config.Identity != nil && config.Managed != nil && config.RecentlyAuthenticated != nil && config.AuthenticationError != nil && config.StepUpPath != nil && config.Error != nil && config.JSON != nil
}

// PasswordLoginConfig connects password authentication to app sessions and presentation.
type PasswordLoginConfig[T any] struct {
	Authenticate func(string, string) (T, bool)
	ProfileID    func(T) string
	SignIn       func(http.ResponseWriter, *http.Request, string) error
	SetAudit     func(*http.Request, T)
	ReturnPath   func(string) string
	Error        func(http.ResponseWriter, *http.Request, string, int)
}

// Login renders a login page or completes its password submission.
func Login[T any](writer http.ResponseWriter, request *http.Request, next string, render func(http.ResponseWriter, *http.Request, any) error, config PasswordLoginConfig[T]) {
	if render == nil {
		http.Error(writer, "password login is unavailable", http.StatusInternalServerError)
		return
	}
	if request.Method == http.MethodGet {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = render(writer, request, next)
		return
	}
	PasswordLoginRequest(writer, request, next, config)
}

// PasswordLoginRequest completes one password login request in Player order.
func PasswordLoginRequest[T any](writer http.ResponseWriter, request *http.Request, next string, config PasswordLoginConfig[T]) {
	if config.Authenticate == nil || config.ProfileID == nil || config.SignIn == nil || config.SetAudit == nil || config.ReturnPath == nil || config.Error == nil {
		http.Error(writer, "password login is unavailable", http.StatusInternalServerError)
		return
	}
	profile, err := PasswordLogin(request.FormValue("name"), request.FormValue("password"), config.Authenticate, func(profile T) error {
		return config.SignIn(writer, request, config.ProfileID(profile))
	})
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			config.Error(writer, request, "invalid credentials", http.StatusUnauthorized)
		} else {
			config.Error(writer, request, "could not create session", http.StatusInternalServerError)
		}
		return
	}
	config.SetAudit(request, profile)
	http.Redirect(writer, request, config.ReturnPath(next), http.StatusSeeOther)
}

// MFAHTTPConfig connects shared MFA endpoints to app identity and persistence.
type MFAHTTPConfig struct {
	RecentlyAuthenticated func(*http.Request, time.Duration) bool
	ReadJSON              func(http.ResponseWriter, *http.Request, any) bool
	Setup                 func(*http.Request) (Enrollment, error)
	Confirm               func(*http.Request, string) error
	MarkStrong            func(*http.Request) error
	Verify                func(*http.Request, string) bool
	Disable               func(*http.Request) error
	Error                 func(http.ResponseWriter, error, int)
	JSON                  func(http.ResponseWriter, any, int)
	WebError              func(http.ResponseWriter, *http.Request, error, int)
	WebConfirmSuccess     func(http.ResponseWriter, *http.Request)
	WebDisableSuccess     func(http.ResponseWriter, *http.Request)
}

// MFAHandlers contains shared MFA HTTP behavior for app-owned routes.
type MFAHandlers struct {
	Setup, Confirm, Disable, WebConfirm, WebDisable http.HandlerFunc
}

// NewMFAHandlers creates Player's shared MFA enrollment lifecycle.
func NewMFAHandlers(config MFAHTTPConfig) (MFAHandlers, error) {
	if !validMFAHTTPConfig(config) {
		return MFAHandlers{}, ErrInvalidConfig
	}
	return MFAHandlers{Setup: mfaSetupHandler(config), Confirm: mfaConfirmHandler(config), Disable: mfaDisableHandler(config), WebConfirm: mfaWebConfirmHandler(config), WebDisable: mfaWebDisableHandler(config)}, nil
}

func validMFAHTTPConfig(config MFAHTTPConfig) bool { //nolint:cyclop // Every required adapter must be present before any MFA side effect.
	return config.RecentlyAuthenticated != nil && config.ReadJSON != nil && config.Setup != nil && config.Confirm != nil && config.MarkStrong != nil && config.Verify != nil && config.Disable != nil && config.Error != nil && config.JSON != nil && config.WebError != nil && config.WebConfirmSuccess != nil && config.WebDisableSuccess != nil
}

func mfaSetupHandler(config MFAHTTPConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct{}
		if !config.ReadJSON(writer, request, &input) {
			return
		}
		if !config.RecentlyAuthenticated(request, 10*time.Minute) {
			config.JSON(writer, map[string]any{"error": "recent authentication required", "stepUpRequired": true}, http.StatusForbidden)
			return
		}
		enrollment, err := config.Setup(request)
		if err != nil {
			config.Error(writer, errors.New("could not create MFA enrollment"), http.StatusInternalServerError)
			return
		}
		config.JSON(writer, map[string]any{"secret": enrollment.Secret, "uri": enrollment.URI, "recoveryCodes": enrollment.RecoveryCodes}, http.StatusCreated)
	}
}

func mfaConfirmHandler(config MFAHTTPConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct {
			Code string `json:"code"`
		}
		if !config.ReadJSON(writer, request, &input) {
			return
		}
		if status, err := applyMFAConfirm(config, request, input.Code); err != nil {
			config.Error(writer, err, status)
			return
		}
		config.JSON(writer, map[string]bool{"enabled": true}, http.StatusOK)
	}
}

func mfaWebConfirmHandler(config MFAHTTPConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if status, err := applyMFAConfirm(config, request, request.FormValue("code")); err != nil {
			config.WebError(writer, request, err, status)
			return
		}
		config.WebConfirmSuccess(writer, request)
	}
}

func applyMFAConfirm(config MFAHTTPConfig, request *http.Request, code string) (int, error) {
	if !config.RecentlyAuthenticated(request, 10*time.Minute) {
		return http.StatusForbidden, errors.New("recent authentication required")
	}
	if err := config.Confirm(request, code); err != nil {
		return http.StatusBadRequest, err
	}
	if err := config.MarkStrong(request); err != nil {
		return http.StatusInternalServerError, errors.New("could not secure current session")
	}
	return 0, nil
}

func mfaDisableHandler(config MFAHTTPConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct {
			Code string `json:"code"`
		}
		if !config.ReadJSON(writer, request, &input) {
			return
		}
		if status, err := applyMFADisable(config, request, input.Code); err != nil {
			config.Error(writer, err, status)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

func mfaWebDisableHandler(config MFAHTTPConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if status, err := applyMFADisable(config, request, request.FormValue("code")); err != nil {
			config.WebError(writer, request, err, status)
			return
		}
		config.WebDisableSuccess(writer, request)
	}
}

func applyMFADisable(config MFAHTTPConfig, request *http.Request, code string) (int, error) {
	if !config.Verify(request, code) {
		return http.StatusUnauthorized, errors.New("invalid authentication code")
	}
	if err := config.Disable(request); err != nil {
		return http.StatusConflict, err
	}
	return 0, nil
}
