package server

import (
	"context"
	"net/http"

	"github.com/MikeO7/kinosail/packages/apihttp"
	sharedoperations "github.com/MikeO7/kinosail/packages/operations"
	settingsops "github.com/MikeO7/kinosail/packages/settings"
)

func registerAdminAPI(mux *http.ServeMux, api apiServices) {
	owner := func(pattern string, handler http.Handler) { mux.Handle(pattern, api.auth.owner(handler)) }
	owner("GET /api/v1/settings", apiSettings(api))
	registerUpdateAPI(owner, api.updates)
	owner("GET /api/v1/agent-connections", apiAgentConnections(api.connections))
	owner("DELETE /api/v1/agent-connections/{id}", apiRevokeAgentConnection(api.connections))
	owner("GET /api/v1/agent-connections/certificate", apiAgentConnectionCertificate(api.connections))
	owner("GET /api/v1/configuration", apiConfiguration(api.settings))
	owner("PUT /api/v1/configuration/{key}", apiChangeConfiguration(api.settings, false))
	owner("DELETE /api/v1/configuration/{key}", apiChangeConfiguration(api.settings, true))
	owner("PUT /api/v1/settings/server", apiSetting(func(input apiSettingInput) error { return api.settings.setName(input.Name) }))
	owner("PUT /api/v1/settings/onboarding", settingsops.SaveOnboardingAPI(api.settings.setOnboardingPending, readJSON, writeJSON, apiError))
	owner("PUT /api/v1/settings/mfa", apiMFARequirement(api.auth))
	owner("PUT /api/v1/settings/session-timeouts", apiSessionTimeouts(api.settings))
	owner("PUT /api/v1/settings/subtitles", apiSubtitleLanguages(api.settings))
	owner("PUT /api/v1/settings/scans", apiSetting(func(input apiSettingInput) error {
		if err := api.settings.setScanFrequency(input.Frequency); err != nil {
			return err
		}
		api.index.SetFrequency(input.Frequency)
		return nil
	}))
	owner("PUT /api/v1/settings/trusted-https", apiTrustedHTTPS(api.settings))
	owner("DELETE /api/v1/settings/trusted-https", apiDisableTrustedHTTPS(api.settings))
	owner("POST /api/v1/libraries", apiLibrarySetting(api.settings, api.index, api.settings.add))
	owner("DELETE /api/v1/libraries", apiLibrarySetting(api.settings, api.index, api.settings.remove))
	owner("GET /api/v1/devices", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, map[string]any{"devices": api.auth.profiles.devices()}, http.StatusOK)
	}))
	owner("DELETE /api/v1/devices/{id}", apiAction(func(request *http.Request) error { return api.auth.profiles.revokeDevice(request.PathValue("id")) }))
	owner("DELETE /api/v1/sessions", apiAction(api.auth.profiles.revokeOtherSessions))
	owner("GET /api/v1/api-keys", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, map[string]any{"keys": api.auth.profiles.apiKeyViews(), "scopes": []string{"admin", "library", "stream"}}, http.StatusOK)
	}))
	owner("POST /api/v1/api-keys", apiCreateKey(api.auth.profiles))
	owner("DELETE /api/v1/api-keys/{id}", apiAction(func(request *http.Request) error { return api.auth.profiles.revokeAPIKey(request.PathValue("id")) }))
	owner("POST /api/v1/tasks/{task}", apiTask(api.index, api.metadata, api.maintenance))
	owner("POST /api/v1/backups", apiBackup(api.backups))
	owner("GET /api/v1/backups", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, api.backups.Status(), http.StatusOK)
	}))
	owner("POST /api/v1/backups/verify", apiBackupVerify(api.backups))
	owner("GET /api/v1/diagnostics", diagnostics(api.settings, api.index, api.auth.profiles, api.auth.audit))
	owner("GET /api/v1/metrics", metrics(api.index, api.auth.profiles, api.auth.audit, api.events))
	owner("GET /api/v1/maintenance", api.maintenance.StatusHandler())
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
	return apihttp.Save(save, errManagedSetting)
}

func apiLibrarySetting(settings *settingsStore, index *libraryIndex, change func(string) error) http.HandlerFunc {
	return apihttp.SaveLibraries(change, func(ctx context.Context) (any, error) {
		if err := index.UpdateRoots(ctx, settings.roots()); err != nil {
			return nil, err
		}
		return settings.snapshot().Libraries, nil
	})
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

func apiTask(index *libraryIndex, metadata *metadataStore, maintenance *maintenanceManager) http.HandlerFunc {
	return sharedoperations.APITaskHandler(operationTasks(index, metadata, maintenance),
		func(writer http.ResponseWriter, _ *http.Request, err error, status int) {
			apiError(writer, err, status)
		},
		func(writer http.ResponseWriter, _ *http.Request) { apiNotFound(writer) })
}
