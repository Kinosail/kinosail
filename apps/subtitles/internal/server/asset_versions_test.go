package server_test

import "testing"

func TestPagesShareOneContentDerivedAssetVersion(t *testing.T) {
	assetContracts.PagesShareOneContentDerivedAssetVersion(t, []string{
		"/", "/account", "/supporter", "/metadata/bulk", "/language",
		"/settings", "/settings/system", "/settings/configuration", "/settings/backups", "/settings/remote-readiness", "/settings/management", "/settings/agent-connections",
	})
}
