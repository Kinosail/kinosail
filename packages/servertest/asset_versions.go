package servertest

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

var versionedAssetReference = regexp.MustCompile(`/static/(app\.css|theme\.js|main\.kinosail\.bundle\.js|supporter\.js)(?:\?v=([^"'\s>]*))?`)

var versionedAssetNames = []string{"app.css", "theme.js", "main.kinosail.bundle.js", "supporter.js"}

func (fixture AssetsFixture) PagesShareOneContentDerivedAssetVersion(t *testing.T, pages []string) {
	t.Helper()
	t.Parallel()

	handler := fixture.NewHandler("", t.TempDir(), true)
	versions := map[string]string{}
	setup := httptest.NewRecorder()
	handler.ServeHTTP(setup, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/setup", nil))
	collectAssetVersions(t, "/setup", setup, versions)
	cookie := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	for _, path := range pages {
		collectAssetVersions(t, path, fixture.WebCall(t, handler, http.MethodGet, path, "", cookie), versions)
	}
	for _, name := range versionedAssetNames {
		version, referenced := versions[name]
		if !referenced {
			t.Fatalf("no page referenced /static/%s", name)
		}
		asset := httptest.NewRecorder()
		handler.ServeHTTP(asset, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/"+name+"?v="+version, nil))
		sum := sha256.Sum256(asset.Body.Bytes())
		if asset.Code != http.StatusOK || hex.EncodeToString(sum[:]) != version {
			t.Fatalf("/static/%s?v=%s = %d, content hash %x", name, version, asset.Code, sum)
		}
		if !strings.Contains(asset.Header().Get("Cache-Control"), "immutable") {
			t.Fatalf("/static/%s?v=%s Cache-Control = %q", name, version, asset.Header().Get("Cache-Control"))
		}
	}
}

func collectAssetVersions(t *testing.T, path string, page *httptest.ResponseRecorder, versions map[string]string) {
	t.Helper()
	if page.Code != http.StatusOK {
		t.Fatalf("%s = %d %q", path, page.Code, page.Body.String())
	}
	matches := versionedAssetReference.FindAllStringSubmatch(page.Body.String(), -1)
	if len(matches) == 0 {
		t.Fatalf("%s references no versioned assets", path)
	}
	for _, match := range matches {
		name, version := match[1], match[2]
		if version == "" {
			t.Fatalf("%s references /static/%s without a version: %q", path, name, page.Body.String())
		}
		if known, seen := versions[name]; seen && known != version {
			t.Fatalf("%s references /static/%s?v=%s while another page used %s", path, name, version, known)
		}
		versions[name] = version
	}
}
