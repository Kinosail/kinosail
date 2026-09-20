package updatecontrol

import (
	"strings"
	"testing"
)

const validManifest = `{"schemaVersion":2,"version":"v1.2.3","updateSchema":2,"stateSchema":1,"minimumStateSchema":1,"configurationSchema":1,"minimumConfigurationSchema":1,"runtimeCommands":["ffmpeg","ffprobe","fpcalc"],"installation":{"schemaVersion":3,"file":"kinosail-native-installation.json","sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","size":100},"artifacts":[{"os":"linux","arch":"amd64","file":"kinosail-core-v1.2.3-linux-amd64.tar.gz","format":"tar.gz","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":1},{"os":"linux","arch":"arm64","file":"kinosail-core-v1.2.3-linux-arm64.tar.gz","format":"tar.gz","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","size":2},{"os":"macos","arch":"amd64","file":"kinosail-core-v1.2.3-macos-amd64.tar.gz","format":"tar.gz","sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","size":3},{"os":"macos","arch":"arm64","file":"kinosail-core-v1.2.3-macos-arm64.tar.gz","format":"tar.gz","sha256":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","size":4},{"os":"windows","arch":"amd64","file":"kinosail-core-v1.2.3-windows-amd64.zip","format":"zip","sha256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","size":5},{"os":"windows","arch":"arm64","file":"kinosail-core-v1.2.3-windows-arm64.zip","format":"zip","sha256":"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff","size":6}]}`

func TestManifestSelectsEverySupportedPlatform(t *testing.T) {
	manifest, err := ParseManifest(strings.NewReader(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct{ goos, arch, file string }{
		{"linux", "amd64", "kinosail-core-v1.2.3-linux-amd64.tar.gz"},
		{"linux", "arm64", "kinosail-core-v1.2.3-linux-arm64.tar.gz"},
		{"darwin", "amd64", "kinosail-core-v1.2.3-macos-amd64.tar.gz"},
		{"darwin", "arm64", "kinosail-core-v1.2.3-macos-arm64.tar.gz"},
		{"windows", "amd64", "kinosail-core-v1.2.3-windows-amd64.zip"},
		{"windows", "arm64", "kinosail-core-v1.2.3-windows-arm64.zip"},
	} {
		artifact, selectErr := manifest.Select(target.goos, target.arch)
		if selectErr != nil || artifact.File != target.file {
			t.Fatalf("%s/%s = %#v, %v", target.goos, target.arch, artifact, selectErr)
		}
	}
}

func TestManifestRejectsAnIncompatibleInstalledSchema(t *testing.T) {
	manifest, err := ParseManifest(strings.NewReader(strings.Replace(validManifest, `"stateSchema":1,"minimumStateSchema":1`, `"stateSchema":2,"minimumStateSchema":2`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if err := manifest.Compatible(1, 1); err == nil {
		t.Fatal("incompatible state schema was accepted")
	}
	if err := manifest.Compatible(2, 1); err != nil {
		t.Fatal(err)
	}
}

func TestManifestRejectsUnknownMissingDuplicateAndOversizedInput(t *testing.T) {
	for name, input := range map[string]string{
		"duplicate version":    strings.Replace(validManifest, `"version":"v1.2.3"`, `"version":"v0.0.1","version":"v1.2.3"`, 1),
		"case alias":           strings.Replace(validManifest, `"version":"v1.2.3"`, `"Version":"v1.2.3","version":"v1.2.3"`, 1),
		"duplicate nested":     strings.Replace(validManifest, `"size":100`, `"size":1,"size":100`, 1),
		"unknown":              strings.Replace(validManifest, `"schemaVersion":2`, `"schemaVersion":2,"channel":"stable"`, 1),
		"old manifest schema":  strings.Replace(validManifest, `"schemaVersion":2`, `"schemaVersion":1`, 1),
		"bad version":          strings.Replace(validManifest, `"version":"v1.2.3"`, `"version":"v1.2.3-beta"`, 1),
		"missing contract":     strings.Replace(validManifest, `"installation":{"schemaVersion":3,"file":"kinosail-native-installation.json","sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","size":100},`, ``, 1),
		"bad contract schema":  strings.Replace(validManifest, `"installation":{"schemaVersion":3`, `"installation":{"schemaVersion":4`, 1),
		"bad contract file":    strings.Replace(validManifest, `kinosail-native-installation.json`, `other.json`, 1),
		"bad contract digest":  strings.Replace(validManifest, `0123456789abcdef`, `0123456789abcdeF`, 1),
		"bad contract size":    strings.Replace(validManifest, `"size":100`, `"size":0`, 1),
		"missing":              strings.Replace(validManifest, `{"os":"windows","arch":"arm64"`, `{"os":"windows","arch":"amd64"`, 1),
		"bad digest":           strings.Replace(validManifest, strings.Repeat("a", 64), strings.Repeat("A", 64), 1),
		"bad size":             strings.Replace(validManifest, `"size":1`, `"size":0`, 1),
		"future source schema": strings.Replace(validManifest, `"minimumStateSchema":1`, `"minimumStateSchema":2`, 1),
		"trailing":             validManifest + `{}`,
		"oversized":            strings.Repeat(" ", manifestLimit+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseManifest(strings.NewReader(input)); err == nil {
				t.Fatal("invalid manifest was accepted")
			}
		})
	}
}
