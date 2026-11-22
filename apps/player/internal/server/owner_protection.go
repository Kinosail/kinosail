package server

import "path/filepath"

var protectedOwnerRoutes = map[string]bool{
	"POST /settings/api-keys": true, "POST /api/v1/api-keys": true,
	"POST /settings/remote/enable": true, "DELETE /api/v1/remote-access/kill": true,
	"POST /settings/management/enable": true, "POST /settings/management/devices": true,
	"POST /api/v1/management-access": true, "POST /api/v1/management-access/devices": true,
	"GET /oauth/authorize": true, "POST /oauth/authorize": true,
	"POST /settings/encrypted-backup": true, "GET /api/v1/backup": true,
	"POST /settings/configuration": true, "PUT /api/v1/configuration/{key}": true,
}

func ownerProtectionRequired(profile viewerProfile, pattern string) bool {
	return profile.Owner && profile.ID != "local-owner" && !profile.Secured() && protectedOwnerRoutes[pattern]
}

func (auth *authentication) mfaRequired(profile viewerProfile) bool {
	return !profile.Owner && auth.settings.requireMFA()
}

func defaultLibraries(mediaRoot string) []string {
	root := filepath.Clean(mediaRoot)
	if mediaRoot != "" && filepath.Dir(root) == root {
		return []string{}
	}
	return []string{"."}
}
