package server

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var assetVersions = contentVersions(map[string][]byte{
	"app.css": applicationStyle, "theme.js": themeJS, "main.kinosail.bundle.js": mainBundle, "supporter.js": supporterJS,
})

var versionedAssetReference = regexp.MustCompile(`/static/(app\.css|theme\.js|main\.kinosail\.bundle\.js|supporter\.js)(\?v=[^"'\s>]*)?`)

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
		return assetURL(name)
	})
}

func versionAssetReferenceBytes(page []byte) []byte {
	return versionedAssetReference.ReplaceAllFunc(page, func(reference []byte) []byte {
		return []byte(versionAssetReferences(string(reference)))
	})
}

func versionedTemplateSource(source string) string {
	return versionAssetReferences(httpguardTemplateSource(source))
}
