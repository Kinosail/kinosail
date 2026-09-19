package remoteaccess

import (
	"errors"
	"net/http"
)

// WriteError preserves one application's localized web error response.
type WriteError func(http.ResponseWriter, *http.Request, string, int)

// KillHTTP disables public access and revokes application authorization.
func KillHTTP(manager *Manager, revoke func() error, writeError WriteError) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if status, err := killResult(manager, revoke); err != nil {
			writeError(writer, request, err.Error(), status)
			return
		}
		http.Redirect(writer, request, "/settings#access", http.StatusSeeOther)
	}
}

// ResetKillHTTP permits public access after the next restart.
func ResetKillHTTP(manager *Manager, writeError WriteError) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := resetResult(manager); err != nil {
			writeError(writer, request, err.Error(), http.StatusConflict)
			return
		}
		http.Redirect(writer, request, "/settings#access", http.StatusSeeOther)
	}
}

// StatusAPI reports public HTTPS readiness.
func StatusAPI[R any](internet *Manager, readiness func(Status) R, write func(http.ResponseWriter, any, int)) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		internetStatus := Status{State: "disabled", Mode: "off"}
		if internet != nil {
			internetStatus = internet.Status()
		}
		write(writer, map[string]any{"directOnly": true, "internet": internetStatus, "securePublic": readiness(internetStatus)}, http.StatusOK)
	}
}

// KillAPI disables public access and revokes authorization for versioned clients.
func KillAPI(manager *Manager, revoke func() error, contract func(string) error, writeError func(http.ResponseWriter, error, int)) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		if status, err := killResult(manager, revoke); err != nil {
			if status == http.StatusConflict {
				err = contract(err.Error())
			}
			writeError(writer, err, status)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

// ResetKillAPI permits public access after restart for versioned clients.
func ResetKillAPI(manager *Manager, contract func(string) error, writeError func(http.ResponseWriter, error, int)) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		if err := resetResult(manager); err != nil {
			writeError(writer, contract(err.Error()), http.StatusConflict)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

// RevokeAuthorization clears every public authorization surface and joins durable errors.
func RevokeAuthorization(revokePasskeys, revokeQuick func(), revokeProfiles func() error, revokeShares func() error) error {
	revokePasskeys()
	revokeQuick()
	profileErr := revokeProfiles()
	var shareErr error
	if revokeShares != nil {
		shareErr = revokeShares()
	}
	return errors.Join(profileErr, shareErr)
}

func killResult(manager *Manager, revoke func() error) (int, error) {
	if manager == nil {
		return http.StatusConflict, errors.New("public HTTPS remote access is not enabled")
	}
	killErr := manager.Kill()
	if revoke() != nil {
		return http.StatusInternalServerError, errors.New("public access is off but public sessions could not be revoked")
	}
	if killErr != nil {
		return http.StatusConflict, killErr
	}
	return 0, nil
}

func resetResult(manager *Manager) error {
	if manager == nil {
		return errors.New("public HTTPS remote access is not enabled")
	}
	return manager.ResetKill()
}
