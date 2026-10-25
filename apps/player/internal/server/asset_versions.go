package server

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"
	"github.com/MikeO7/kinosail/packages/quickconnect"
	"github.com/MikeO7/kinosail/packages/webassets"
)

var assetVersions = contentVersions(map[string][]byte{
	"app.css": applicationStyle, "theme.js": themeJS, "main.kinosail.bundle.js": mainBundle, "supporter.js": supporterJS,
	"htmx.min.js": htmx, "player.js": playerJS, "pwa.js": pwaJS,
	"supporter.css": supporterCSS, "quick-connect.js": quickConnectJS, "connect.js": connectJS,
	"quick-connect-scan.js": quickConnectScanJS, "qr-decoder.js": qrDecoderJS, "public-login.js": webassets.PublicLogin,
	"passkeys.js": passkeysJS, "manrope.woff2": webassets.Manrope, "icon.svg": icon, "tmdb-logo.svg": tmdbLogo,
	"icon-192.png": icon192, "icon-512.png": icon512, "icon-maskable-512.png": iconMaskable512,
	"apple-touch-icon.png": appleTouchIcon, "cinema-backdrop.jpg": cinemaBackdrop, "hls.min.js": hlsJS,
})

var (
	versionedAssetReference = regexp.MustCompile(`/static/([A-Za-z0-9._-]+)(\?v=[^"'\s>]*)?`)
	hlsAssetReference       = regexp.MustCompile(`/static/hls\.min\.js(\?v=[^"'\s>]*)?`)
	hlsVersion              = contentVersions(map[string][]byte{"hls.min.js": hlsJS})["hls.min.js"]
)

func versionHLSReferences(script []byte) []byte {
	return hlsAssetReference.ReplaceAll(script, []byte("/static/hls.min.js?v="+hlsVersion))
}

func contentVersions(assets map[string][]byte) map[string]string {
	versions := make(map[string]string, len(assets))
	for name, content := range assets {
		sum := sha256.Sum256(content)
		versions[name] = hex.EncodeToString(sum[:])
	}
	return versions
}

func assetURL(name string) string {
	return "/static/" + name + "?v=" + assetVersions[name]
}

func versionAssetReferences(source string) string {
	return versionedAssetReference.ReplaceAllStringFunc(source, func(reference string) string {
		name := strings.TrimPrefix(reference, "/static/")
		if cut := strings.IndexByte(name, '?'); cut >= 0 {
			name = name[:cut]
		}
		if _, known := assetVersions[name]; !known {
			return reference
		}
		return assetURL(name)
	})
}

func versionedTemplateSource(source string) string {
	return versionAssetReferences(httpguardTemplateSource(source))
}

func quickConnectPage(page quickconnect.Page) quickconnect.Page {
	page.ThemeScript, page.Stylesheet, page.Icon = assetURL("theme.js"), assetURL("app.css"), assetURL("icon.svg")
	return page
}

func bulkMetadataPage(page sharedmetadata.BulkPage) sharedmetadata.BulkPage {
	page.ThemeScript, page.Stylesheet = assetURL("theme.js"), assetURL("app.css")
	return page
}
