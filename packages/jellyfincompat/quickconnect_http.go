package jellyfincompat

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/quickconnect"
)

// QuickConnectApproval owns Player's Viewer and recent-auth checks.
func QuickConnectApproval[Profile any](
	public func(*http.Request) bool,
	current func(*http.Request) Profile,
	profileID func(Profile) string,
	recent func(*http.Request) bool,
	approve func(Profile, string, bool) error,
) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if public(request) {
			WriteQuickConnectApproval(writer, request, ErrPublicApproval)
			return
		}
		userID, userValid := StrictQuery(request.URL.Query(), "userId", 256)
		code, codeValid := StrictQuery(request.URL.Query(), "code", 64)
		code, codeValid = normalizeQuickConnectCode(code, codeValid)
		if !codeValid {
			WriteQuickConnectApproval(writer, request, quickconnect.ErrInvalidCode)
			return
		}
		profile := current(request)
		if !userValid || userID != "" && userID != profileID(profile) {
			WriteQuickConnectApproval(writer, request, ErrViewerUnavailable)
			return
		}
		WriteQuickConnectApproval(writer, request, approve(profile, code, recent(request)))
	}
}

// QuickConnectAuthentication owns one-use consumption and response projection.
func QuickConnectAuthentication[Profile any](
	polls *httpguard.Limiter,
	consume func(string) (string, Profile, error),
	audit func(*http.Request, Profile),
	project func(Profile) User,
) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		AuthenticateQuickConnect(writer, request, polls, func(secret string) (Authentication, error) {
			token, profile, err := consume(secret)
			if err != nil {
				return Authentication{}, err
			}
			audit(request, profile)
			return Authentication{Token: token, User: project(profile)}, nil
		})
	}
}

// StrictQuery accepts at most one bounded case-insensitive query value.
func StrictQuery(values url.Values, name string, maximum int) (string, bool) {
	candidates := foldedValues(values, name)
	if len(candidates) > 1 {
		return "", false
	}
	if len(candidates) == 0 {
		return "", true
	}
	return candidates[0], validOptionalValue(candidates[0], maximum)
}

func validQuickConnectRequest(request quickconnect.Request) bool {
	return request.Device != "" && request.DeviceID != "" && request.Client != "" && request.Version != "" &&
		validOptionalValue(request.Device, 80) && validOptionalValue(request.DeviceID, 128) &&
		validOptionalValue(request.Client, 80) && validOptionalValue(request.Version, 40)
}

func strictMediaBrowserValue(request *http.Request, name string) (string, bool) {
	values := make([]string, 0, 1)
	for _, authorization := range []string{request.Header.Get("Authorization"), request.Header.Get("X-Emby-Authorization")} {
		for _, part := range strings.Split(authorization, ",") {
			key, value, found := strings.Cut(strings.TrimSpace(part), "=")
			if scheme, parameter, separated := strings.Cut(key, " "); separated && strings.EqualFold(scheme, "MediaBrowser") {
				key = parameter
			}
			if found && strings.EqualFold(key, name) {
				values = append(values, strings.Trim(strings.TrimSpace(value), `"`))
			}
		}
	}
	return singleValue(values)
}

func singleValue(values []string) (string, bool) {
	if len(values) != 1 {
		return "", false
	}
	return values[0], true
}

func normalizeQuickConnectCode(code string, valid bool) (string, bool) {
	code = strings.TrimSpace(code)
	if !valid || len(code) != 6 {
		return "", false
	}
	for _, character := range code {
		if character < '0' || character > '9' {
			return "", false
		}
	}
	return code, true
}

func validQuickConnectSecret(secret string) bool {
	if len(secret) < 4 || len(secret) > 128 || !strings.HasPrefix(secret, "qc_") {
		return false
	}
	for _, character := range secret[3:] {
		if (character < 'A' || character > 'Z') && (character < '2' || character > '7') {
			return false
		}
	}
	return true
}
