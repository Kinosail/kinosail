// Package productapi owns Player's shared product API projections and metadata mutations.
package productapi

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/apihttp"
	"github.com/MikeO7/kinosail/packages/identitycore"
)

// MeState contains app-owned values projected by the shared current-viewer endpoint.
type MeState struct {
	Server, ServerID, Language, LanguagePreference string
	Viewer                                         identitycore.Profile
	SSO                                            bool
	Languages                                      any
}

type viewerResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Rating      string   `json:"rating"`
	AccessStart string   `json:"accessStart,omitempty"`
	AccessEnd   string   `json:"accessEnd,omitempty"`
	Owner       bool     `json:"owner"`
	Downloads   bool     `json:"downloads"`
	Transcode   bool     `json:"transcode"`
	Remote      bool     `json:"remote"`
	Libraries   []string `json:"libraries"`
}

type meResponse struct {
	Server             string         `json:"server"`
	ServerID           string         `json:"serverId,omitempty"`
	Viewer             viewerResponse `json:"viewer"`
	SSO                bool           `json:"sso"`
	Language           string         `json:"language"`
	LanguagePreference string         `json:"languagePreference"`
	Languages          any            `json:"languages"`
}

// Me serves Player's canonical current-viewer projection.
func Me(state func(*http.Request) MeState) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		input := state(request)
		viewer := input.Viewer
		apihttp.WriteJSON(writer, meResponse{
			Server: input.Server, ServerID: input.ServerID,
			Viewer: viewerResponse{
				ID: viewer.ID, Name: viewer.Name, Rating: viewer.Rating,
				AccessStart: viewer.AccessStart, AccessEnd: viewer.AccessEnd,
				Owner:     viewer.Permits("admin", viewer.Owner),
				Downloads: viewer.Permits("download", viewer.Owner || viewer.Downloads),
				Transcode: viewer.Permits("stream", viewer.Owner || viewer.Transcode),
				Remote:    viewer.Permits("library", viewer.Owner || viewer.Remote),
				Libraries: viewer.Libraries,
			},
			SSO: input.SSO, Language: input.Language,
			LanguagePreference: input.LanguagePreference, Languages: input.Languages,
		}, http.StatusOK)
	}
}
