package jellyfincompat

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/quickconnect"
)

const (
	// Version is the Player-authoritative Jellyfin compatibility version.
	Version             = "12.0.0"
	maxCredentialsBytes = 1 << 20
)

var (
	// Password-login errors map app-owned identity outcomes to the Jellyfin contract.
	ErrPublicPasswordLogin = errors.New("public password login is disabled; use Quick Connect") //nolint:staticcheck // Product term starts with a capital.
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrCredentialRateLimit = errors.New("too many login attempts")
	ErrQuickConnectMFA     = errors.New("two-factor authentication requires Quick Connect") //nolint:staticcheck // Product term starts with a capital.
	ErrCreateSession       = errors.New("could not create session")
	ErrViewerUnavailable   = errors.New("Viewer Profile is not available")              //nolint:staticcheck // Product term is capitalized.
	ErrPublicApproval      = errors.New("public Quick Connect approval is unavailable") //nolint:staticcheck // Product term starts with a capital.
)

var jellyfinPrefixes = []string{
	"/System/", "/system/", "/QuickConnect/", "/Users", "/Library/", "/Auth/", "/UserViews", "/UserItems/",
	"/UserPlayedItems/", "/UserFavoriteItems/", "/Items", "/Shows/", "/Videos/", "/Audio/", "/Sessions/",
	"/Branding/", "/MediaSegments/", "/Playback/",
}

// Credentials is the bounded Jellyfin password-login document.
type Credentials struct{ Username, Pw, Password string }

// User contains app-owned profile facts used by Jellyfin projections.
type User struct {
	ID, Name, ServerID, ServerName                  string
	HasPassword, Owner, Disabled, Downloads, Remote bool
}

// Authentication is one completed app-owned Jellyfin login.
type Authentication struct {
	Token string
	User  User
}

// CoreHandlers contains app-owned adapters for the stable Jellyfin entry routes.
type CoreHandlers struct {
	SystemInfo, Views, BitrateTest, Authenticate, User, Users, CreateAuthKey, AuthKeys, Logout http.HandlerFunc
}

// QuickConnectHandlers contains app-owned operations for the stable Jellyfin Quick Connect routes.
type QuickConnectHandlers struct{ Start, Status, Approve, Authenticate http.HandlerFunc }

// Path reports whether a request path belongs to the Jellyfin compatibility API.
func Path(path string) bool {
	for _, prefix := range jellyfinPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// RegisterCore registers the stable Jellyfin entry route contract.
func RegisterCore(mux *http.ServeMux, handlers CoreHandlers) {
	mux.HandleFunc("GET /System/Info/Public", handlers.SystemInfo)
	mux.HandleFunc("GET /system/info/public", handlers.SystemInfo)
	mux.HandleFunc("GET /System/Info", handlers.SystemInfo)
	mux.HandleFunc("GET /Library/MediaFolders", handlers.Views)
	mux.HandleFunc("GET /Users/Public", Value([]any{}))
	mux.HandleFunc("GET /Branding/Configuration", Value(map[string]any{}))
	mux.HandleFunc("GET /Playback/BitrateTest", handlers.BitrateTest)
	mux.HandleFunc("POST /Users/AuthenticateByName", handlers.Authenticate)
	mux.HandleFunc("GET /Users/Me", handlers.User)
	mux.HandleFunc("GET /Users/{id}", handlers.User)
	mux.HandleFunc("GET /Users", handlers.Users)
	mux.HandleFunc("POST /Auth/Keys", handlers.CreateAuthKey)
	mux.HandleFunc("GET /Auth/Keys", handlers.AuthKeys)
	mux.HandleFunc("POST /Sessions/Capabilities", Empty)
	mux.HandleFunc("POST /Sessions/Capabilities/Full", Empty)
	mux.HandleFunc("POST /Sessions/Logout", handlers.Logout)
}

// RegisterQuickConnect registers the stable Jellyfin Quick Connect route contract.
func RegisterQuickConnect(mux *http.ServeMux, handlers QuickConnectHandlers) {
	mux.HandleFunc("GET /QuickConnect/Enabled", Value(true))
	mux.HandleFunc("GET /QuickConnect/Initiate", handlers.Start)
	mux.HandleFunc("POST /QuickConnect/Initiate", handlers.Start)
	mux.HandleFunc("GET /QuickConnect/Connect", handlers.Status)
	mux.HandleFunc("POST /QuickConnect/Authorize", handlers.Approve)
	mux.HandleFunc("POST /Users/AuthenticateWithQuickConnect", handlers.Authenticate)
}

// SystemInfo projects the stable Jellyfin server identity.
func SystemInfo(id, name, address string, configured bool) map[string]any {
	return map[string]any{
		"Id": id, "ServerName": name, "Version": Version, "ProductName": "Jellyfin Server", "OperatingSystem": "Linux",
		"StartupWizardCompleted": configured, "LocalAddress": address,
	}
}

// UserDTO projects app-owned profile facts to the Jellyfin user contract.
func UserDTO(user User) map[string]any {
	return map[string]any{
		"Id": user.ID, "Name": user.Name, "ServerId": user.ServerID, "ServerName": user.ServerName, "HasPassword": user.HasPassword,
		"Configuration": map[string]any{"GroupedFolders": []string{"Movies", "Shows"}, "PlayDefaultAudioTrack": true, "SubtitleMode": "Default", "EnableNextEpisodeAutoPlay": true},
		"Policy":        map[string]any{"AuthenticationProviderId": "Kinosail", "PasswordResetProviderId": "Kinosail", "IsAdministrator": user.Owner, "IsDisabled": user.Disabled, "EnableMediaPlayback": true, "EnableAudioPlaybackTranscoding": false, "EnableVideoPlaybackTranscoding": false, "EnableContentDownloading": user.Owner || user.Downloads, "EnableRemoteAccess": user.Owner || user.Remote},
	}
}

// AuthenticationResult projects one completed Jellyfin login.
func AuthenticationResult(authentication Authentication) map[string]any {
	user := authentication.User
	return map[string]any{"AccessToken": authentication.Token, "ServerId": user.ServerID, "User": UserDTO(user), "SessionInfo": map[string]any{"UserId": user.ID, "UserName": user.Name}}
}

// JSON writes one uncached Jellyfin JSON response.
func JSON(writer http.ResponseWriter, value any) { JSONStatus(writer, value, http.StatusOK) }

// JSONStatus writes one uncached Jellyfin JSON response with an explicit status.
func JSONStatus(writer http.ResponseWriter, value any, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

// Value returns a handler for one fixed Jellyfin JSON value.
func Value(value any) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) { JSON(writer, value) }
}

// Empty writes one successful response without a body.
func Empty(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) }

// PasswordAuthentication validates and translates one app-owned password login.
func PasswordAuthentication(public func(*http.Request) bool, login func(*http.Request, Credentials) (Authentication, error)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if public(request) {
			passwordError(writer, ErrPublicPasswordLogin)
			return
		}
		credentials, valid := readCredentials(request)
		if !valid {
			http.Error(writer, ErrInvalidCredentials.Error(), http.StatusBadRequest)
			return
		}
		authentication, err := login(request, credentials)
		if err != nil {
			passwordError(writer, err)
			return
		}
		JSON(writer, AuthenticationResult(authentication))
	}
}

func readCredentials(request *http.Request) (Credentials, bool) {
	var credentials Credentials
	if request.Body == nil || httpguard.DecodeJSON(request.Body, maxCredentialsBytes, &credentials, false) != nil || len(credentials.Username) > 256 || len(credentials.Pw) > 1024 || len(credentials.Password) > 1024 {
		return Credentials{}, false
	}
	if credentials.Pw == "" {
		credentials.Pw = credentials.Password
	}
	return credentials, true
}

func passwordError(writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrPublicPasswordLogin), errors.Is(err, ErrQuickConnectMFA):
		status = http.StatusForbidden
	case errors.Is(err, ErrInvalidCredentials):
		status = http.StatusUnauthorized
	case errors.Is(err, ErrCredentialRateLimit):
		writer.Header().Set("Retry-After", "60")
		status = http.StatusTooManyRequests
	default:
		err = ErrCreateSession
	}
	http.Error(writer, err.Error(), status)
}

// QuickConnectRequest extracts the required bounded Jellyfin device identity.
func QuickConnectRequest(request *http.Request, remote bool) (quickconnect.Request, bool) {
	device, deviceValid := strictMediaBrowserValue(request, "Device")
	deviceID, deviceIDValid := strictMediaBrowserValue(request, "DeviceId")
	client, clientValid := strictMediaBrowserValue(request, "Client")
	version, versionValid := strictMediaBrowserValue(request, "Version")
	cleanDevice := identitycore.CleanDeviceName(device)
	if !validOptionalValue(device, 80) {
		cleanDevice = device
	}
	identity := quickconnect.Request{Device: cleanDevice, DeviceID: deviceID, Client: client, Version: version, Numeric: true, Remote: remote}
	return identity, deviceValid && deviceIDValid && clientValid && versionValid && device != "" && deviceID != "" && client != "" && version != ""
}

// QuickConnectDTO projects shared authorization state to the Jellyfin contract.
func QuickConnectDTO(secret string, pending quickconnect.Connection) map[string]any {
	return map[string]any{
		"Authenticated": pending.Approved, "Secret": secret, "Code": pending.Code, "DateAdded": pending.Created,
		"DeviceId": pending.DeviceID, "DeviceName": pending.Device, "AppName": pending.Client, "AppVersion": pending.Version,
	}
}

// StartQuickConnect validates identity, applies the protocol rate limit, and creates one request.
func StartQuickConnect(writer http.ResponseWriter, request *http.Request, connections *quickconnect.Broker, starts *httpguard.Limiter, remote bool) {
	identity, valid := QuickConnectRequest(request, remote)
	if !valid {
		http.Error(writer, "device identity is required", http.StatusBadRequest)
		return
	}
	if !validQuickConnectRequest(identity) {
		http.Error(writer, "device identity is invalid", http.StatusBadRequest)
		return
	}
	if !starts.Allow(httpguard.RemoteHost(request.RemoteAddr), 20) {
		writer.Header().Set("Retry-After", "60")
		http.Error(writer, "too many Quick Connect requests", http.StatusTooManyRequests) //nolint:staticcheck // Product term starts with a capital.
		return
	}
	secret, pending, err := connections.Create(identity)
	if err != nil {
		http.Error(writer, "could not create Quick Connect request", http.StatusInternalServerError) //nolint:staticcheck // Product term starts with a capital.
		return
	}
	JSON(writer, QuickConnectDTO(secret, pending))
}

// QuickConnectStatus applies the protocol poll limit and writes current request state.
func QuickConnectStatus(writer http.ResponseWriter, request *http.Request, connections *quickconnect.Broker, polls *httpguard.Limiter) {
	if !polls.Allow(httpguard.RemoteHost(request.RemoteAddr), 120) {
		writer.Header().Set("Retry-After", "60")
		http.Error(writer, "too many Quick Connect polls", http.StatusTooManyRequests) //nolint:staticcheck // Product term starts with a capital.
		return
	}
	secret, valid := StrictQuery(request.URL.Query(), "secret", 128)
	if !valid || !validQuickConnectSecret(secret) {
		http.NotFound(writer, request)
		return
	}
	pending, found := connections.Status(secret)
	if !found {
		http.NotFound(writer, request)
		return
	}
	JSON(writer, QuickConnectDTO(secret, pending))
}

// AuthenticateQuickConnect validates one secret and delegates app-owned session creation.
func AuthenticateQuickConnect(writer http.ResponseWriter, request *http.Request, polls *httpguard.Limiter, authenticate func(string) (Authentication, error)) {
	if !polls.Allow(httpguard.RemoteHost(request.RemoteAddr), 120) {
		writer.Header().Set("Retry-After", "60")
		http.Error(writer, "too many Quick Connect polls", http.StatusTooManyRequests) //nolint:staticcheck // Product term starts with a capital.
		return
	}
	var input struct{ Secret string }
	if err := httpguard.DecodeRequestJSON(writer, request, &input); err != nil {
		JSONStatus(writer, map[string]string{"error": err.Error()}, http.StatusBadRequest)
		return
	}
	if !validQuickConnectSecret(input.Secret) {
		http.NotFound(writer, request)
		return
	}
	authentication, err := authenticate(input.Secret)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	JSON(writer, AuthenticationResult(authentication))
}

// WriteQuickConnectApproval maps app-owned approval outcomes to Jellyfin responses.
func WriteQuickConnectApproval(writer http.ResponseWriter, request *http.Request, err error) {
	switch {
	case err == nil:
		JSON(writer, true)
	case errors.Is(err, ErrViewerUnavailable), errors.Is(err, quickconnect.ErrRemoteViewer):
		http.Error(writer, err.Error(), http.StatusForbidden)
	default:
		http.NotFound(writer, request)
	}
}
