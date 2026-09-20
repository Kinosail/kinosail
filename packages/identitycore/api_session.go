package identitycore

import (
	"errors"
	"net/http"
	"strings"
)

// APISessionConfig binds Player's API sign-in flow to app-owned state.
type APISessionConfig struct {
	Public              func(*http.Request) bool
	ReadJSON            func(http.ResponseWriter, *http.Request, any) bool
	AllowCredential     func(string, string) bool
	Authenticate        func(string, string) (Profile, bool)
	VerifySecondFactor  func(string, string) bool
	CredentialSucceeded func(string)
	CreateSession       func(string, string) (string, error)
	CreateStrongSession func(string, string, bool) (string, error)
	MFARequired         func(Profile) bool
	SetAudit            func(*http.Request, Profile)
	Error               func(http.ResponseWriter, error, int)
	JSON                func(http.ResponseWriter, any, int)
}

type apiSessionInput struct{ Name, Password, Device, Code string }

// CreateAPISession applies Player's credential, MFA, and session order.
func CreateAPISession(writer http.ResponseWriter, request *http.Request, config APISessionConfig) {
	if !validAPISessionConfig(config) {
		http.Error(writer, "API sign-in is unavailable", http.StatusInternalServerError)
		return
	}
	if config.Public(request) {
		config.Error(writer, errors.New("public password login is disabled; use a passkey or Quick Connect"), http.StatusForbidden)
		return
	}
	input, ok := readAPISessionInput(writer, request, config)
	if !ok {
		return
	}
	profile, ok := authenticateAPISession(writer, request, input, config)
	if !ok {
		return
	}
	config.CredentialSucceeded(input.Name)
	token, err := createAPISessionToken(config, profile, input.Device)
	if err != nil {
		config.Error(writer, errors.New("could not create session"), http.StatusInternalServerError)
		return
	}
	config.SetAudit(request, profile)
	config.JSON(writer, map[string]any{
		"token": token, "expiresIn": 2592000,
		"mfaEnrollmentRequired": config.MFARequired(profile) && !profile.Secured(),
		"passkey":               map[string]any{"configured": len(profile.Passkeys) > 0, "usedForSignIn": false},
	}, http.StatusCreated)
}

func readAPISessionInput(writer http.ResponseWriter, request *http.Request, config APISessionConfig) (apiSessionInput, bool) {
	input := apiSessionInput{request.FormValue("name"), request.FormValue("password"), request.FormValue("device"), request.FormValue("code")}
	if strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") && !config.ReadJSON(writer, request, &input) {
		return apiSessionInput{}, false
	}
	return input, true
}

func authenticateAPISession(writer http.ResponseWriter, request *http.Request, input apiSessionInput, config APISessionConfig) (Profile, bool) {
	if !config.AllowCredential(request.RemoteAddr, input.Name) {
		writer.Header().Set("Retry-After", "60")
		config.Error(writer, errors.New("too many login attempts"), http.StatusTooManyRequests)
		return Profile{}, false
	}
	profile, ok := config.Authenticate(input.Name, input.Password)
	if !ok {
		config.Error(writer, errors.New("invalid credentials"), http.StatusUnauthorized)
		return Profile{}, false
	}
	return profile, verifyAPISessionFactor(writer, input, profile, config)
}

func verifyAPISessionFactor(writer http.ResponseWriter, input apiSessionInput, profile Profile, config APISessionConfig) bool {
	if profile.TOTPSecret == "" {
		return true
	}
	if input.Code == "" {
		config.JSON(writer, map[string]any{"error": "authentication code required", "mfaRequired": true}, http.StatusUnauthorized)
		return false
	}
	if config.VerifySecondFactor(profile.ID, input.Code) {
		return true
	}
	config.Error(writer, errors.New("invalid authentication code"), http.StatusUnauthorized)
	return false
}

func createAPISessionToken(config APISessionConfig, profile Profile, device string) (string, error) {
	return config.CreateStrongSession(profile.ID, device, false)
}

func validAPISessionConfig(config APISessionConfig) bool { //nolint:cyclop // All adapters must exist before credential or session side effects.
	return config.Public != nil && config.ReadJSON != nil && config.AllowCredential != nil && config.Authenticate != nil && config.VerifySecondFactor != nil && config.CredentialSucceeded != nil && config.CreateSession != nil && config.CreateStrongSession != nil && config.MFARequired != nil && config.SetAudit != nil && config.Error != nil && config.JSON != nil
}
