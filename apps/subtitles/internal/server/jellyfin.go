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
	auth      *authentication
	index     *libraryIndex
	probe     *mediaProbe
	progress  *progressStore
	hls       *hlsManager
	lists     *listStore
	downloads *downloadManager
	keyGrants sharedjellyfin.Grants
	plays     sync.Map
}

type jellyfinPlaySession struct {
	itemID          string
	plan            PlaybackPlan
	profileID       string
	profileRevision uint64
	public          bool
	expires         time.Time
}

func (session jellyfinPlaySession) JellyfinPlan() PlaybackPlan { return session.plan }
func (session jellyfinPlaySession) JellyfinItemID() string     { return session.itemID }
func (session jellyfinPlaySession) JellyfinPublic() bool       { return session.public }
func (session jellyfinPlaySession) JellyfinExpires() time.Time {
	return session.expires
}

func (session jellyfinPlaySession) JellyfinProfile() (string, uint64) {
	return session.profileID, session.profileRevision
}

func registerJellyfin(mux *http.ServeMux, settings *settingsStore, index *libraryIndex, progress *progressStore, lists *listStore, auth *authentication, quickConnect *quickConnectBroker, probe *mediaProbe, hls *hlsManager, downloads *downloadManager) {
	api := &jellyfinAPI{id: settings.jellyfinID(), settings: settings, index: index, progress: progress, lists: lists, auth: auth, probe: probe, hls: hls, downloads: downloads}
	sharedjellyfin.RegisterCore(mux, api.jellyfinCoreHandlers())
	quickConnect.registerJellyfin(mux, api)
	api.registerItems(mux)
	api.registerPlayback(mux)
	mux.HandleFunc("GET /MediaSegments/{id}", markerlogic.JellyfinHandler(api.jellyfinMarkers, jellyfinJSON))
}

func (api *jellyfinAPI) jellyfinCoreHandlers() sharedjellyfin.CoreHandlers {
	return sharedjellyfin.CoreHandlers{
		SystemInfo: api.systemInfo, Views: api.views, BitrateTest: jellyfinBitrateTest,
		Authenticate: sharedjellyfin.PasswordAuthentication(publicInternetRequest, api.passwordLogin), User: api.user,
		Users: api.auth.owner(http.HandlerFunc(api.users)).ServeHTTP, CreateAuthKey: api.auth.owner(http.HandlerFunc(api.createAuthKey)).ServeHTTP,
		AuthKeys: api.auth.owner(http.HandlerFunc(api.authKeys)).ServeHTTP, Logout: api.logout,
	}
}

func jellyfinPath(path string) bool { return sharedjellyfin.Path(path) }

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

func (api *jellyfinAPI) systemInfo(writer http.ResponseWriter, request *http.Request) {
	api.identityModule().SystemInfo(writer, request)
}

func (api *jellyfinAPI) passwordLogin(request *http.Request, credentials sharedjellyfin.Credentials) (sharedjellyfin.Authentication, error) {
	return api.identityModule().PasswordLogin(request, credentials)
}

func (api *jellyfinAPI) identityModule() sharedjellyfin.Identity[viewerProfile] {
	return sharedjellyfin.Identity[viewerProfile]{
		Configured: api.auth.profiles.hasProfiles, ServerName: api.settings.serverName, ServerID: api.id,
		Current: currentViewer, Public: publicInternetRequest, Secure: secureRequest,
		ProfileID: func(profile viewerProfile) string { return profile.ID }, Project: api.jellyfinUser,
		Authenticate: api.auth.profiles.authenticate, AllowLogin: api.auth.allowCredentialLogin,
		RequiresMFA: func(profile viewerProfile) bool { return profile.TOTPSecret != "" || api.auth.settings.requireMFA() },
		Audit:       setAuditViewer, Compatibility: compatibilityProfile, LoginSucceeded: api.auth.credentialLoginSucceeded,
		SignOut: api.auth.profiles.signOut, CreateSession: api.auth.profiles.createCompatibilitySession,
	}
}

func jellyfinJSON(writer http.ResponseWriter, value any) { sharedjellyfin.JSON(writer, value) }
