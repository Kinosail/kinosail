package server_test

import "testing"

func TestPagesShareOneContentDerivedAssetVersion(t *testing.T) {
	assetContracts.PagesShareOneContentDerivedAssetVersion(t, []string{
		"/", "/?view=music", "/account", "/supporter", "/quick-connect", "/metadata/bulk", "/offline-downloads", "/language",
		"/settings", "/settings/system", "/settings/configuration", "/settings/backups", "/settings/media-shares", "/settings/remote-readiness", "/settings/management", "/settings/agent-connections",
	})
}
