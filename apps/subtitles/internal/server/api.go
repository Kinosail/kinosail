package server

import (
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail/packages/apihttp"
	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/homeassistant"
	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/remoteaccess"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

type clientItem = catalogapi.ClientItem

type apiServices struct {
	index         *libraryIndex
	progress      *progressStore
	lists         *listStore
	auth          *authentication
	settings      *settingsStore
	hls           *hlsManager
	probe         *mediaProbe
	metadata      *metadataStore
	subtitles     *subtitleProvider
	rooms         *watchRoomAdapter
	backups       *backupManager
	downloads     *downloadManager
	maintenance   *maintenanceManager
	imports       *viewingImportManager
	connections   *mcpConnections
	internet      *remoteaccess.Manager
	trusted       *trustedhttps.Manager
	shares        *mediaShareStore
	quick         *quickConnectBroker
	supporter     *supporterProgram
	updates       *updateChecker
	homeAssistant *homeAssistantIntegration
	authURL       string
	events        *liveEventHub
}

func (auth *authentication) createAPISession(writer http.ResponseWriter, request *http.Request) {
	config := auth.apiSessionConfig()
	if identitycore.ManagementDeviceKey(request) != "" {
		config.CreateSession = func(id, name string) (string, error) {
			return auth.profiles.sessionModule().CreateForRequest(request, id, name, false, false, "")
		}
		config.CreateStrongSession = func(id, name string, browser bool) (string, error) {
			return auth.profiles.sessionModule().CreateForRequest(request, id, name, browser, true, "")
		}
	}
	identitycore.CreateAPISession(writer, request, config)
}

func (auth *authentication) apiSessionConfig() identitycore.APISessionConfig {
	return identitycore.APISessionConfig{
		Public: publicInternetRequest, ReadJSON: readJSON,
		AllowCredential: auth.allowCredentialLogin, Authenticate: auth.profiles.authenticate,
		VerifySecondFactor: auth.profiles.verifySecondFactor, CredentialSucceeded: auth.credentialLoginSucceeded,
		CreateSession: auth.profiles.createSession, CreateStrongSession: auth.profiles.createStrongSession,
		MFARequired: func(profile viewerProfile) bool { return profile.Owner || auth.settings.requireMFA() },
		SetAudit:    setAuditViewer, Error: apiError, JSON: writeJSON,
	}
}

func (auth *authentication) createAPISetup(writer http.ResponseWriter, request *http.Request) {
	if auth.profiles.hasProfiles() {
		apiError(writer, errors.New("server setup is already complete"), http.StatusConflict)
		return
	}
	var input struct {
		Name, Password, Device string
		TOTP                   bool
		AutomaticUpdates       *bool `json:"automaticUpdates"`
	}
	if !readJSON(writer, request, &input) {
		return
	}
	automaticUpdates := true
	if input.AutomaticUpdates != nil {
		automaticUpdates = *input.AutomaticUpdates
	}
	profile, enrollment, err := auth.createFirstOwner(input.Name, input.Password, input.TOTP, automaticUpdates)
	if err != nil {
		apiError(writer, err, ownerSetupStatus(err))
		return
	}
	token, err := auth.profiles.createStrongSession(profile.ID, input.Device, false)
	if err != nil {
		apiError(writer, errors.New("could not create session"), http.StatusInternalServerError)
		return
	}
	setAuditViewer(request, profile)
	result := map[string]any{"token": token, "expiresIn": 2592000, "mfaEnrollmentRequired": !profile.Secured()}
	if enrollment != nil {
		result["totp"] = enrollmentJSON(*enrollment)
	}
	writeJSON(writer, result, http.StatusCreated)
}

func (auth *authentication) deleteAPISession(writer http.ResponseWriter, request *http.Request) {
	if err := auth.profiles.signOut(request); err != nil {
		apiError(writer, errors.New("could not end session"), http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func registerAPI(mux *http.ServeMux, api apiServices) {
	apihttp.Register(mux, api)
}

func (api apiServices) RegisterFoundationAPI(mux *http.ServeMux) {
	api.events.Register(mux, liveEventAccess(api.auth))
	mux.HandleFunc("GET /api/v1/library", catalogapi.Library(func(request *http.Request) (catalog.Result, error) {
		return browseLibrary(request, api.index, api.progress, api.lists)
	}, api.progress.ClientItem))
	registerMediaAPI(mux, api.index, api.progress, api.lists, api.auth)
}

func (api apiServices) RegisterDownloadsAPI(mux *http.ServeMux) {
	api.downloads.registerAPI(mux, api.index)
}

func (api apiServices) RegisterAdminAPI(mux *http.ServeMux) {
	registerAdminAPI(mux, api)
}

func (api apiServices) RegisterSupporterAPI(mux *http.ServeMux) {
	registerSupporterAPI(mux, api.auth, api.supporter)
}

func (api apiServices) RegisterProductAPI(mux *http.ServeMux) {
	registerProductAPI(mux, api)
	api.homeAssistant.Register(mux, api.auth.owner, func(writer http.ResponseWriter, request *http.Request, view homeassistant.Approval) error {
		return executeCSRFTemplate(homeAssistantApprovalView, writer, request, view)
	})
}

func apiStoreStatus(err error) int {
	return apihttp.StoreStatus(err, errInvalidProgressState)
}

var (
	readJSON    = apihttp.ReadJSON
	writeJSON   = apihttp.WriteJSON
	apiError    = apihttp.Error
	apiNotFound = apihttp.NotFound
	apiRouting  = apihttp.Routing
)

func toClientItem(request *http.Request, progress *progressStore, viewer viewerProfile, item library.Item) clientItem {
	return catalogapi.ProjectViewerItem(item, progress.Get(request, item.ID), viewer)
}
