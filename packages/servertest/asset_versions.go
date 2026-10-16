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

var staticAssetReference = regexp.MustCompile(`/static/([A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)*)(?:\?v=([^"'\s>&)]*))?`)

var versionedAssetNames = []string{"app.css", "theme.js", "main.kinosail.bundle.js", "supporter.js", "icon.svg"}

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
		if _, referenced := versions[name]; !referenced {
			t.Fatalf("no page referenced /static/%s", name)
		}
	}
	for name, version := range versions {
		assertVersionedAsset(t, handler, cookie, name, version)
	}
}

func CurrentAssetURL(t *testing.T, handler http.Handler, path string) string {
	t.Helper()
	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	if asset.Code != http.StatusOK {
		t.Fatalf("%s = %d", path, asset.Code)
	}
	sum := sha256.Sum256(asset.Body.Bytes())
	return path + "?v=" + hex.EncodeToString(sum[:])
}

func assertVersionedAsset(t *testing.T, handler http.Handler, cookie *http.Cookie, name, version string) {
	t.Helper()
	signed := func(target string) *http.Request {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
		request.AddCookie(cookie)
		return request
	}
	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, signed("/static/"+name+"?v="+version))
	sum := sha256.Sum256(asset.Body.Bytes())
	if asset.Code != http.StatusOK || hex.EncodeToString(sum[:]) != version || asset.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("/static/%s?v=%s = %d, content hash %x, cache %q", name, version, asset.Code, sum, asset.Header().Get("Cache-Control"))
	}
	tag := asset.Header().Get("ETag")
	if tag != `"`+version+`"` {
		t.Fatalf("/static/%s ETag = %q", name, tag)
	}
	revalidation := signed("/static/" + name)
	revalidation.Header.Set("If-None-Match", `W/"stale", `+tag)
	unchanged := httptest.NewRecorder()
	handler.ServeHTTP(unchanged, revalidation)
	if unchanged.Code != http.StatusNotModified || unchanged.Body.Len() != 0 || unchanged.Header().Get("ETag") != tag || unchanged.Header().Get("Cache-Control") != "public, max-age=86400" {
		t.Fatalf("/static/%s revalidation = %d, %d bytes, %v", name, unchanged.Code, unchanged.Body.Len(), unchanged.Header())
	}
	stale := httptest.NewRecorder()
	handler.ServeHTTP(stale, signed("/static/"+name+"?v=stale"))
	if stale.Code != http.StatusOK || stale.Header().Get("Cache-Control") != "public, max-age=86400" {
		t.Fatalf("/static/%s with another version = %d %q", name, stale.Code, stale.Header().Get("Cache-Control"))
	}
}

func collectAssetVersions(t *testing.T, path string, page *httptest.ResponseRecorder, versions map[string]string) {
	t.Helper()
	if page.Code != http.StatusOK {
		t.Fatalf("%s = %d %q", path, page.Code, page.Body.String())
	}
	matches := staticAssetReference.FindAllStringSubmatch(page.Body.String(), -1)
	if len(matches) == 0 {
		t.Fatalf("%s references no static assets", path)
	}
	for _, match := range matches {
		name, version := match[1], match[2]
		if strings.HasPrefix(name, "supporter/badges/") {
			continue
		}
		if version == "" {
			t.Fatalf("%s references /static/%s without a version: %q", path, name, page.Body.String())
		}
		if known, seen := versions[name]; seen && known != version {
			t.Fatalf("%s references /static/%s?v=%s while another page used %s", path, name, version, known)
		}
		versions[name] = version
	}
}
