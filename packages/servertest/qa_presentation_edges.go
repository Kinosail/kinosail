package servertest

import (
	"strings"
	"testing"

	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"
)

// AssertDLNALabelExplainsUnconfiguredAndConfiguredStates preserves the Player regression against the supplied app bindings.
func AssertDLNALabelExplainsUnconfiguredAndConfiguredStates(t *testing.T, label func() string, configure func(string, string)) {
	if got := label(); got != "Set KINOSAIL_DLNA_URL to enable" {
		t.Fatalf("unconfigured DLNA label = %q", got)
	}
	configure("http://media.local:8200", "")
	if got := label(); got != "Disabled" {
		t.Fatalf("disabled DLNA label = %q", got)
	}
	configure("http://media.local:8200", "token")
	if got := label(); got != "Enabled" {
		t.Fatalf("enabled DLNA label = %q", got)
	}
}

// AssertHardwareSupportAcceptsAutomaticAndKnownBackends preserves the Player regression against the supplied app bindings.
func AssertHardwareSupportAcceptsAutomaticAndKnownBackends(t *testing.T, supports func(string) bool) {
	if !supports("auto") || !supports("vaapi") || supports("nvenc") {
		t.Fatal("hardware support classification is incorrect")
	}
}

// AssertTMDBCastValidationBoundsProviderData preserves the Player regression against the supplied app bindings.
func AssertTMDBCastValidationBoundsProviderData(t *testing.T) {
	if !sharedmetadata.ValidTMDBCast([]sharedmetadata.TMDBCastMember{{Name: "Actor", Character: "Role", ProfilePath: "/actor.jpg"}}) {
		t.Fatal("valid TMDB cast was rejected")
	}
	for _, cast := range [][]sharedmetadata.TMDBCastMember{
		make([]sharedmetadata.TMDBCastMember, 1001),
		{{Name: strings.Repeat("n", 201)}},
		{{Name: "Actor", ProfilePath: "https://images.example/actor.jpg"}},
	} {
		if sharedmetadata.ValidTMDBCast(cast) {
			t.Fatal("invalid TMDB cast was accepted")
		}
	}
}

// AssertPlayerFormattingUsesStableHumanReadableBoundaries preserves the Player regression against the supplied app bindings.
func AssertPlayerFormattingUsesStableHumanReadableBoundaries(t *testing.T, byteSize func(int64) string, oneOf func(string, ...string) bool) {
	AssertByteSizeBoundaries(t, byteSize)
	if !oneOf("hevc", "h264", "hevc") || oneOf("vp9", "h264", "hevc") {
		t.Fatal("oneOf classification is incorrect")
	}
}

// AssertByteSizeBoundaries checks formatting in apps that no longer own allowlist helpers.
func AssertByteSizeBoundaries(t *testing.T, byteSize func(int64) string) {
	for size, want := range map[int64]string{0: "0 B", 1024: "1.0 KB", 1024 * 1024: "1.0 MB", 1024 * 1024 * 1024: "1.0 GB"} {
		if got := byteSize(size); got != want {
			t.Errorf("byteSize(%d) = %q, want %q", size, got, want)
		}
	}
}
