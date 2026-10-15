package server_test

import "testing"

func TestPagesShareOneContentDerivedAssetVersion(t *testing.T) {
	assetContracts.PagesShareOneContentDerivedAssetVersion(t, []string{
		"/", "/?view=music", "/account", "/supporter", "/metadata/bulk", "/offline-downloads", "/language",
		"/settings", "/settings/system", "/settings/configuration", "/settings/backups", "/settings/remote-readiness", "/settings/management", "/settings/agent-connections",
	})
}
