package server_test

import "testing"

func TestPagesShareOneContentDerivedAssetVersion(t *testing.T) {
	assetContracts.PagesShareOneContentDerivedAssetVersion(t, []string{
		"/", "/account", "/supporter", "/language",
		"/settings", "/settings/system", "/settings/configuration", "/settings/backups", "/settings/management", "/settings/agent-connections",
	})
}
