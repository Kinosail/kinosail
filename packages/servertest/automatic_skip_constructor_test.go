package servertest

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAutomaticSkipConstructorPreservesAppBindings(t *testing.T) {
	for _, option := range []struct {
		name     string
		unescape bool
		segments int
	}{{"Player", true, 10}, {"Subtitles", false, 8}} {
		t.Run(option.name, func(t *testing.T) {
			for _, jellyfin := range []bool{false, true} {
				assertAutomaticSkipConstructor(t, jellyfin, option.unescape, option.segments)
			}
		})
	}
}

func assertAutomaticSkipConstructor(t *testing.T, jellyfin, unescape bool, segments int) {
	t.Helper()
	input := AutomaticSkipConfig{Lifecycle: t.Context(), MediaDir: "media", DataDir: "data", CacheDir: "cache", FFprobe: "probe", FFmpeg: "encoder", Jellyfin: jellyfin}
	configureCalls, factoryCalls := 0, 0
	handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusAccepted) })
	fixture := NewAutomaticSkipFixture(func(value AutomaticSkipConfig) AutomaticSkipConfig {
		configureCalls++
		if value != input {
			t.Fatal("constructor changed a fixture field before app conversion")
		}
		return value
	}, func(value AutomaticSkipConfig) http.Handler {
		factoryCalls++
		if value != input || jellyfin {
			t.Fatal("plain factory received changed fields or a Jellyfin request")
		}
		return handler
	}, func(test *testing.T, value AutomaticSkipConfig) http.Handler {
		factoryCalls++
		if test != t || value != input || !jellyfin {
			t.Fatal("Jellyfin factory received changed fields or a plain request")
		}
		return handler
	}, constructorTOTP(t), unescape, segments)
	response := httptest.NewRecorder()
	fixture.New(t, input).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if response.Code != http.StatusAccepted || configureCalls != 1 || factoryCalls != 1 {
		t.Fatal("constructor did not return the selected app handler exactly once")
	}
	assertAutomaticSkipOptions(t, fixture, unescape, segments)
}

func constructorTOTP(t *testing.T) func(*testing.T, string, time.Time) string {
	t.Helper()
	return func(test *testing.T, secret string, at time.Time) string {
		if test != t || secret != "fixture-secret" || !at.Equal(time.Unix(123, 0)) {
			t.Fatal("constructor changed TOTP callback arguments")
		}
		return "fixture-code"
	}
}

func assertAutomaticSkipOptions(t *testing.T, fixture AutomaticSkipFixture, unescape bool, segments int) {
	t.Helper()
	if fixture.UnescapeSubtitleQuery != unescape || fixture.CopiedHLSSegments != segments {
		t.Fatal("constructor changed app query or observed segment options")
	}
	if fixture.TOTP(t, "fixture-secret", time.Unix(123, 0)) != "fixture-code" {
		t.Fatal("constructor did not forward the TOTP callback")
	}
}
