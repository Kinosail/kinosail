package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

func (store *settingsStore) auditSnapshot(request *http.Request) map[string]string { //nolint:cyclop,funlen // Each mutable settings projection is intentionally allowlisted.
	path := request.URL.Path
	store.mu.RLock()
	defer store.mu.RUnlock()
	switch path {
	case "/settings/server", "/api/v1/settings/server":
		return map[string]string{"name": store.value.Name}
	case "/settings/navigation", "/api/v1/settings/navigation":
		return map[string]string{"items": strings.Join(store.value.Navigation, ",")}
	case "/settings/mfa", "/api/v1/settings/mfa":
		return map[string]string{"required": strconv.FormatBool(store.value.RequireMFA)}
	case "/settings/session-timeouts", "/api/v1/settings/session-timeouts":
		inactive, absolute := sessionTimeoutHours(store.value)
		return map[string]string{"inactiveHours": strconv.FormatFloat(inactive, 'f', -1, 64), "absoluteHours": strconv.FormatFloat(absolute, 'f', -1, 64)}
	case "/settings/playback", "/api/v1/settings/playback":
		return map[string]string{"mode": normalizePlayback(store.value.PlaybackMode), "autoplay": strconv.FormatBool(store.value.Autoplay), "subtitles": store.value.Subtitles, "autoSkip": strings.Join(store.value.AutoSkip, ",")}
	case "/settings/transcoder", "/api/v1/settings/transcoder":
		return map[string]string{"quality": store.value.Transcoder, "codec": normalizeCodecSetting(store.value.Codec), "accelerator": normalizeAccelerator(store.value.Accelerator), "toneMap": strconv.FormatBool(store.value.ToneMap)}
	case "/settings/transcoder/test", "/api/v1/transcoder/test":
		return map[string]string{"status": store.transcoderCheck.Status, "codec": store.transcoderCheck.Codec, "accelerator": store.transcoderCheck.Accelerator, "stage": store.transcoderCheck.Stage}
	case "/settings/subtitles", "/onboarding/subtitles/language", "/api/v1/settings/subtitles":
		languages := subtitleLanguages(store.value)
		return map[string]string{"language": languages[0], "languages": strings.Join(languages, ",")}
	case "/settings/scans", "/onboarding/subtitles/scans", "/api/v1/settings/scans":
		return map[string]string{"frequency": store.value.ScanFrequency}
	case "/settings/dlna", "/api/v1/settings/dlna":
		return map[string]string{"enabled": strconv.FormatBool(store.value.DLNAToken != "")}
	case "/settings/jellyfin", "/onboarding/jellyfin", "/api/v1/settings/jellyfin":
		return map[string]string{"enabled": strconv.FormatBool(store.value.JellyfinCompatibility && trustedHTTPSConfigured(store.config.String("tls.duckdns")))}
	case "/settings/updates", "/onboarding/updates", "/api/v1/settings/updates":
		return map[string]string{"automatic": strconv.FormatBool(store.value.UpdateChecks)}
	case "/settings/trusted-https", "/settings/trusted-https/disable", "/onboarding/trusted-https", "/api/v1/settings/trusted-https":
		config, _ := trustedhttps.Parse(store.config.String("tls.duckdns"))
		return map[string]string{"configured": strconv.FormatBool(config != (trustedhttps.Config{})), "provider": config.ProviderName()}
	case "/settings/libraries", "/settings/libraries/remove", "/onboarding/subtitles/libraries", "/onboarding/subtitles/libraries/remove", "/api/v1/libraries":
		return map[string]string{"libraries": strings.Join(store.value.Libraries, ",")}
	case "/settings/configuration", "/settings/configuration/reset":
		return configurationAuditValue(store, request.FormValue("key"))
	default:
		if strings.HasPrefix(path, "/api/v1/configuration/") {
			return configurationAuditValue(store, strings.TrimPrefix(path, "/api/v1/configuration/"))
		}
	}
	return nil
}

func configurationAuditValue(store *settingsStore, key string) map[string]string {
	if key == oidcConfigurationKey {
		issuer := store.config.Public("integrations.oidc.issuer")
		return map[string]string{"key": key, "source": string(issuer.Source), "configured": strconv.FormatBool(issuer.Configured), "issuer": truncate(issuer.Value, 240), "clientId": truncate(store.config.String("integrations.oidc.client_id"), 240), "redirectUrl": truncate(store.config.String("integrations.oidc.redirect_url"), 240)}
	}
	field := store.config.Public(key)
	if field.Key == "" {
		return nil
	}
	result := map[string]string{"key": key, "source": string(field.Source), "configured": strconv.FormatBool(field.Configured)}
	if !field.Secret {
		result["value"] = truncate(field.Value, 240)
	}
	return result
}
