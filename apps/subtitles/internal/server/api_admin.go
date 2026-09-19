package server

import (
	"context"
	"net/http"

	"github.com/MikeO7/kinosail/packages/apihttp"
	sharedoperations "github.com/MikeO7/kinosail/packages/operations"
	settingsops "github.com/MikeO7/kinosail/packages/settings"
)

type apiProfileInput struct {
	Name, Password, Rating, AccessStart, AccessEnd string
	Libraries                                      []string
	Owner, Downloads, Transcode, Remote            bool
}

func (input apiProfileInput) policy() profilePolicy {
	return profilePolicy{
		Downloads: input.Downloads, Transcode: input.Transcode, Remote: input.Remote,
		Rating: input.Rating, AccessStart: input.AccessStart, AccessEnd: input.AccessEnd, Libraries: input.Libraries,
	}
}

type apiProfile struct {
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
	MFAEnabled  bool     `json:"mfaEnabled"`
}

func publicProfiles(profiles []viewerProfile) []apiProfile {
	result := make([]apiProfile, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, apiProfile{profile.ID, profile.Name, profile.Rating, profile.AccessStart, profile.AccessEnd, profile.Owner, profile.Downloads, profile.Transcode, profile.Remote, profile.Libraries, profile.Secured()})
	}
	return result
}

func registerAdminAPI(mux *http.ServeMux, api apiServices) { //nolint:funlen // The explicit administrative route registry is intentionally centralized.
	owner := func(pattern string, handler http.Handler) { mux.Handle(pattern, api.auth.owner(handler)) }
	owner("GET /api/v1/settings", apiSettings(api))
	registerUpdateAPI(owner, api.updates)
	owner("GET /api/v1/agent-connections", apiAgentConnections(api.connections))
	owner("DELETE /api/v1/agent-connections/{id}", apiRevokeAgentConnection(api.connections))
	owner("GET /api/v1/agent-connections/certificate", apiAgentConnectionCertificate(api.connections))
	owner("GET /api/v1/hardware", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, api.settings.hardware, http.StatusOK)
	}))
	owner("POST /api/v1/transcoder/test", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		result := api.settings.runTranscoderCheck(request.Context())
		status := http.StatusOK
		if result.Status != "passed" {
			status = http.StatusServiceUnavailable
		}
		writeJSON(writer, result, status)
	}))
	owner("GET /api/v1/configuration", apiConfiguration(api.settings))
	owner("PUT /api/v1/configuration/{key}", apiChangeConfiguration(api.settings, false))
	owner("DELETE /api/v1/configuration/{key}", apiChangeConfiguration(api.settings, true))
	owner("PUT /api/v1/settings/server", apiSetting(func(input apiSettingInput) error { return api.settings.setName(input.Name) }))
	owner("PUT /api/v1/settings/navigation", apiSetting(func(input apiSettingInput) error { return api.settings.setNavigation(input.Items) }))
	owner("PUT /api/v1/settings/onboarding", settingsops.SaveOnboardingAPI(api.settings.setOnboardingPending, readJSON, writeJSON, apiError))
	owner("PUT /api/v1/settings/mfa", apiMFARequirement(api.auth))
	owner("PUT /api/v1/settings/session-timeouts", apiSessionTimeouts(api.settings))
	owner("PUT /api/v1/settings/playback", apiSetting(func(input apiSettingInput) error {
		return api.settings.setPlayback(input.Mode, input.Autoplay, input.Subtitles, input.AutoSkip)
	}))
	owner("PUT /api/v1/settings/transcoder", apiSetting(func(input apiSettingInput) error {
		return api.settings.setTranscoder(input.Quality, input.Codec, input.Accelerator, input.ToneMap)
	}))
	owner("PUT /api/v1/settings/subtitles", apiSubtitleLanguages(api.settings))
	owner("PUT /api/v1/settings/scans", apiSetting(func(input apiSettingInput) error {
		if err := api.settings.setScanFrequency(input.Frequency); err != nil {
			return err
		}
		api.index.SetFrequency(input.Frequency)
		return nil
	}))
	owner("PUT /api/v1/settings/dlna", apiSetting(func(input apiSettingInput) error { return api.settings.setDLNA(input.Enabled) }))
	owner("PUT /api/v1/settings/jellyfin", apiSetting(func(input apiSettingInput) error { return api.settings.setJellyfinCompatibility(input.Enabled) }))
	owner("PUT /api/v1/settings/home-assistant", api.homeAssistant.SettingHandler())
	owner("PUT /api/v1/settings/trusted-https", apiTrustedHTTPS(api.settings))
	owner("DELETE /api/v1/settings/trusted-https", apiDisableTrustedHTTPS(api.settings))
	owner("POST /api/v1/libraries", apiLibrarySetting(api.settings, api.index, api.settings.add))
	owner("DELETE /api/v1/libraries", apiLibrarySetting(api.settings, api.index, api.settings.remove))
	owner("GET /api/v1/profiles", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, map[string]any{"profiles": publicProfiles(api.auth.profiles.list())}, http.StatusOK)
	}))
	owner("POST /api/v1/profiles", apiCreateProfile(api.auth.profiles))
	owner("PUT /api/v1/profiles/{id}", apiUpdateProfile(api.auth.profiles))
	owner("DELETE /api/v1/profiles/{id}", apiDeleteProfile(api.auth.profiles))
	owner("PUT /api/v1/profiles/{id}/password", apiPassword(api.auth.profiles))
	owner("GET /api/v1/devices", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, map[string]any{"devices": api.auth.profiles.devices()}, http.StatusOK)
	}))
	owner("DELETE /api/v1/devices/{id}", apiAction(func(request *http.Request) error { return api.auth.profiles.revokeDevice(request.PathValue("id")) }))
	owner("DELETE /api/v1/sessions", apiAction(api.auth.profiles.revokeOtherSessions))
	owner("GET /api/v1/api-keys", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, map[string]any{"keys": api.auth.profiles.apiKeyViews(), "scopes": []string{"admin", "download", "library", "stream", "write"}}, http.StatusOK)
	}))
	owner("POST /api/v1/api-keys", apiCreateKey(api.auth.profiles))
	owner("DELETE /api/v1/api-keys/{id}", apiAction(func(request *http.Request) error { return api.auth.profiles.revokeAPIKey(request.PathValue("id")) }))
	owner("POST /api/v1/tasks/{task}", apiTask(api.index, api.hls, api.metadata, api.maintenance))
	owner("POST /api/v1/backups", apiBackup(api.backups))
	owner("GET /api/v1/backups", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, api.backups.Status(), http.StatusOK)
	}))
	owner("POST /api/v1/backups/verify", apiBackupVerify(api.backups))
	owner("POST /api/v1/media-shares", http.HandlerFunc(api.shares.CreateHTTP))
	owner("GET /api/v1/media-shares", http.HandlerFunc(api.shares.ListHTTP))
	owner("DELETE /api/v1/media-shares/{id}", http.HandlerFunc(api.shares.RevokeHTTP))
	owner("GET /api/v1/diagnostics", diagnostics(api.settings, api.index, api.auth.profiles, api.hls, api.auth.audit))
	owner("GET /api/v1/metrics", metrics(api.index, api.auth.profiles, api.hls, api.auth.audit, api.imports, api.events, api.rooms))
	owner("GET /api/v1/maintenance", http.HandlerFunc(api.maintenance.serveStatus))
}

func apiBackupVerify(backups *backupManager) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		if err := backups.Verify(); err != nil {
			apiError(writer, err, http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

func apiBackup(backups *backupManager) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		if err := backups.WriteNow(); err != nil {
			apiError(writer, err, http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

type apiSettingInput struct {
	Name, Mode, Subtitles, Quality, Codec, Accelerator, Language, Preference, Frequency string
	AutoSkip                                                                            []string
	Items                                                                               []string
	Autoplay, ToneMap, Enabled                                                          bool
}

func apiSetting(save func(apiSettingInput) error) http.HandlerFunc {
	return apihttp.Save(save, errManagedSetting, errJellyfinTrustedHTTPSRequired)
}

func apiLibrarySetting(settings *settingsStore, index *libraryIndex, change func(string) error) http.HandlerFunc {
	return apihttp.SaveLibraries(change, func(ctx context.Context) (any, error) {
		if err := index.UpdateRoots(ctx, settings.roots()); err != nil {
			return nil, err
		}
		return settings.snapshot().Libraries, nil
	})
}

func apiCreateProfile(profiles *profileStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input apiProfileInput
		if !readJSON(writer, request, &input) {
			return
		}
		id, err := profiles.addProfile(input.Name, input.Password, input.Owner, input.policy())
		if err != nil {
			apiError(writer, err, http.StatusBadRequest)
			return
		}
		writeJSON(writer, map[string]string{"id": id, "name": input.Name}, http.StatusCreated)
	}
}

func apiUpdateProfile(profiles *profileStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input apiProfileInput
		if !readJSON(writer, request, &input) {
			return
		}
		if err := profiles.setProfile(request.PathValue("id"), input.Owner, input.policy()); err != nil { //nolint:contextcheck // Authorization changes must finish after validation.
			apiError(writer, err, http.StatusBadRequest)
			return
		}
		writeJSON(writer, map[string]string{"status": "saved"}, http.StatusOK)
	}
}

func apiDeleteProfile(profiles *profileStore) http.HandlerFunc {
	return apiAction(func(request *http.Request) error { return profiles.removeProfile(request.PathValue("id")) })
}

func apiPassword(profiles *profileStore) http.HandlerFunc { //nolint:contextcheck // A validated credential rotation must durably finish after request cancellation.
	return func(writer http.ResponseWriter, request *http.Request) { //nolint:contextcheck // The validated credential rotation must durably finish after disconnect.
		var input struct{ Password string }
		if !readJSON(writer, request, &input) {
			return
		}
		if err := profiles.resetPassword(request.PathValue("id"), input.Password); err != nil { //nolint:contextcheck // Credential and session changes form one durable commit.
			apiError(writer, err, http.StatusBadRequest)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

func apiCreateKey(profiles *profileStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct{ Name, Scopes string }
		if !readJSON(writer, request, &input) {
			return
		}
		secret, err := profiles.createAPIKey(currentViewer(request), input.Name, input.Scopes)
		if err != nil {
			apiError(writer, err, http.StatusBadRequest)
			return
		}
		writeJSON(writer, map[string]string{"name": input.Name, "secret": secret}, http.StatusCreated)
	}
}

func apiAction(action func(*http.Request) error) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := action(request); err != nil {
			apiError(writer, err, http.StatusBadRequest)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

func apiTask(index *libraryIndex, hls *hlsManager, metadata *metadataStore, maintenance *maintenanceManager) http.HandlerFunc {
	return sharedoperations.APITaskHandler(operationTasks(index, hls, metadata, maintenance),
		func(writer http.ResponseWriter, _ *http.Request, err error, status int) {
			apiError(writer, err, status)
		},
		func(writer http.ResponseWriter, _ *http.Request) { apiNotFound(writer) })
}
