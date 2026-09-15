package server

import (
	"net/http"
	"sync"
	"time"

	sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"
	markerlogic "github.com/MikeO7/kinosail/packages/markers"
)

type jellyfinAPI struct {
	id        string
	settings  *settingsStore
	index     *libraryIndex
	progress  *progressStore
	lists     *listStore
	auth      *authentication
	probe     *mediaProbe
	hls       *hlsManager
	downloads *downloadManager
	plays     sync.Map
	keyGrants sharedjellyfin.Grants
}

type jellyfinPlaySession struct {
	itemID          string
	profileID       string
	profileRevision uint64
	expires         time.Time
	plan            PlaybackPlan
	public          bool
}

func (session jellyfinPlaySession) JellyfinItemID() string { return session.itemID }
func (session jellyfinPlaySession) JellyfinProfile() (string, uint64) {
	return session.profileID, session.profileRevision
}
func (session jellyfinPlaySession) JellyfinExpires() time.Time { return session.expires }
func (session jellyfinPlaySession) JellyfinPublic() bool       { return session.public }
func (session jellyfinPlaySession) JellyfinPlan() PlaybackPlan { return session.plan }

func registerJellyfin(mux *http.ServeMux, settings *settingsStore, index *libraryIndex, progress *progressStore, lists *listStore, auth *authentication, quickConnect *quickConnectBroker, probe *mediaProbe, hls *hlsManager, downloads *downloadManager) {
	api := &jellyfinAPI{id: settings.jellyfinID(), settings: settings, index: index, progress: progress, lists: lists, auth: auth, probe: probe, hls: hls, downloads: downloads}
	sharedjellyfin.RegisterCore(mux, sharedjellyfin.CoreHandlers{
		SystemInfo: api.systemInfo, Views: api.views, BitrateTest: jellyfinBitrateTest,
		Authenticate: sharedjellyfin.PasswordAuthentication(publicInternetRequest, api.passwordLogin), User: api.user,
		Users: api.auth.owner(http.HandlerFunc(api.users)).ServeHTTP, CreateAuthKey: api.auth.owner(http.HandlerFunc(api.createAuthKey)).ServeHTTP,
		AuthKeys: api.auth.owner(http.HandlerFunc(api.authKeys)).ServeHTTP, Logout: api.logout,
	})
	quickConnect.registerJellyfin(mux, api)
	api.registerItems(mux)
	api.registerPlayback(mux)
	mux.HandleFunc("GET /MediaSegments/{id}", markerlogic.JellyfinHandler(api.jellyfinMarkers, jellyfinJSON))
}

func jellyfinPath(path string) bool { return sharedjellyfin.Path(path) }

func (api *jellyfinAPI) systemInfo(writer http.ResponseWriter, request *http.Request) {
	api.identityModule().SystemInfo(writer, request)
}

func (api *jellyfinAPI) passwordLogin(request *http.Request, credentials sharedjellyfin.Credentials) (sharedjellyfin.Authentication, error) {
	return api.identityModule().PasswordLogin(request, credentials)
}

func (api *jellyfinAPI) user(writer http.ResponseWriter, request *http.Request) {
	api.identityModule().User(writer, request)
}

func (api *jellyfinAPI) userDTO(profile viewerProfile) map[string]any {
	return sharedjellyfin.UserDTO(api.jellyfinUser(profile))
}

func (api *jellyfinAPI) jellyfinUser(profile viewerProfile) sharedjellyfin.User {
	return sharedjellyfin.User{ID: profile.ID, Name: profile.Name, ServerID: api.id, ServerName: api.settings.serverName(), HasPassword: profile.Credential != "", Owner: profile.Owner, Disabled: profile.Disabled || profile.SCIMDeleted, Downloads: profile.Downloads, Remote: profile.Remote}
}

func (api *jellyfinAPI) logout(writer http.ResponseWriter, request *http.Request) {
	api.identityModule().Logout(writer, request)
}

func (api *jellyfinAPI) identityModule() sharedjellyfin.Identity[viewerProfile] {
	return sharedjellyfin.Identity[viewerProfile]{
		ServerID: api.id, ServerName: api.settings.serverName, Configured: api.auth.profiles.hasProfiles,
		Secure: secureRequest, Public: publicInternetRequest, Current: currentViewer,
		ProfileID: func(profile viewerProfile) string { return profile.ID }, Project: api.jellyfinUser,
		AllowLogin: api.auth.allowCredentialLogin, Authenticate: api.auth.profiles.authenticate,
		RequiresMFA:   func(profile viewerProfile) bool { return profile.TOTPSecret != "" || api.auth.settings.requireMFA() },
		Compatibility: compatibilityProfile, Audit: setAuditViewer, LoginSucceeded: api.auth.credentialLoginSucceeded,
		CreateSession: api.auth.profiles.createCompatibilitySession, SignOut: api.auth.profiles.signOut,
	}
}

func jellyfinJSON(writer http.ResponseWriter, value any) { sharedjellyfin.JSON(writer, value) }
