package jellyfincompat

import (
	"net/http"
	"unicode"
)

// Identity contains the app adapters for Player's Jellyfin identity flow.
type Identity[Profile any] struct {
	ServerID       string
	ServerName     func() string
	Configured     func() bool
	Secure         func(*http.Request) bool
	Public         func(*http.Request) bool
	Current        func(*http.Request) Profile
	ProfileID      func(Profile) string
	Project        func(Profile) User
	AllowLogin     func(string, string) bool
	Authenticate   func(string, string) (Profile, bool)
	RequiresMFA    func(Profile) bool
	Compatibility  func(Profile) Profile
	Audit          func(*http.Request, Profile)
	LoginSucceeded func(string)
	CreateSession  func(string, string) (string, error)
	SignOut        func(*http.Request) error
}

// SystemInfo writes Player's Jellyfin server identity.
func (identity Identity[Profile]) SystemInfo(writer http.ResponseWriter, request *http.Request) {
	scheme := "http"
	if identity.Secure(request) {
		scheme = "https"
	}
	JSON(writer, SystemInfo(identity.ServerID, identity.ServerName(), scheme+"://"+request.Host, identity.Configured()))
}

// PasswordLogin runs Player's ordered compatibility-login policy.
func (identity Identity[Profile]) PasswordLogin(request *http.Request, credentials Credentials) (Authentication, error) {
	if !identity.AllowLogin(request.RemoteAddr, credentials.Username) {
		return Authentication{}, ErrCredentialRateLimit
	}
	profile, valid := identity.Authenticate(credentials.Username, credentials.Pw)
	if !valid {
		return Authentication{}, ErrInvalidCredentials
	}
	if identity.RequiresMFA(profile) {
		return Authentication{}, ErrQuickConnectMFA
	}
	profile = identity.Compatibility(profile)
	identity.Audit(request, profile)
	identity.LoginSucceeded(credentials.Username)
	token, err := identity.CreateSession(identity.ProfileID(profile), request.UserAgent())
	if err != nil {
		return Authentication{}, err
	}
	return Authentication{Token: token, User: identity.Project(profile)}, nil
}

// User writes the current profile or rejects a mismatched path identifier.
func (identity Identity[Profile]) User(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if !validOptionalValue(id, 256) {
		http.NotFound(writer, request)
		return
	}
	profile := identity.Current(request)
	if id != "" && id != identity.ProfileID(profile) {
		http.NotFound(writer, request)
		return
	}
	JSON(writer, UserDTO(identity.Project(profile)))
}

// Logout revokes the current session before it reports success.
func (identity Identity[Profile]) Logout(writer http.ResponseWriter, request *http.Request) {
	if err := identity.SignOut(request); err != nil {
		http.Error(writer, "could not end session", http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func validOptionalValue(value string, maximum int) bool {
	if len(value) > maximum {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
