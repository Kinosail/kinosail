package server

import (
	"net/http"
	"strings"
)

const onboardingConnectionHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="theme-color" content="#0b0d0b"><title>Choose a Server address · Kinosail Player</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=skeleton-3"></head><body class="settings-page onboarding-page"><a class="skip" href="#main">Skip to content</a><main id="main" class="settings-shell onboarding-shell">{{template "onboardingProgress"}}<header class="settings-intro"><span class="eyebrow">Step 2 of 4 · Devices</span><h1>Make every device feel at home.</h1><p>Choose the address your phones, TVs, and browsers will trust. Kinosail stays on your network unless you separately enable remote access.</p></header><div class="settings-flow"><section class="wide"><h2>Trusted HTTPS with DuckDNS</h2>{{template "configurationSource" .TrustedControl}}{{if .Trusted.Configured}}<p role="status">Saved for <code>{{.Trusted.Hostname}}</code> — restart Kinosail to request the certificate.</p>{{end}}<p>Use a free DuckDNS name and Let's Encrypt certificate to avoid certificate warnings on phones, TVs, and browsers. This publishes the private LAN address in public DNS and the hostname in certificate-transparency logs, but it does not open a router port or relay media. The DuckDNS token can update every hostname in that account; Kinosail stores it in the protected secrets file and never shows it here.</p><details class="onboarding-disclosure" id="trusted-https-configuration"><summary>{{if .Trusted.Configured}}Review trusted HTTPS setup{{else}}Set up trusted HTTPS{{end}}</summary><form action="/onboarding/trusted-https" method="post"><fieldset {{if .TrustedControl.Managed}}disabled aria-disabled="true"{{end}}><label>DuckDNS subdomain <span><input name="domain" value="{{.Trusted.Domain}}" placeholder="my-kinosail" maxlength="63" required>.duckdns.org</span></label><label>DuckDNS token <input type="password" name="token" autocomplete="off" placeholder="{{if .Trusted.TokenSet}}Leave blank to keep the current token{{else}}Paste your DuckDNS token{{end}}" maxlength="128" {{if not .Trusted.TokenSet}}required{{end}}></label><label>Kinosail LAN address <input name="address" inputmode="decimal" value="{{.Trusted.Address}}" placeholder="192.168.1.10" maxlength="15" required></label><label><input type="checkbox" name="termsAccepted" value="true" required> Allow DNS validation and accept the Let's Encrypt subscriber agreement</label><button>Save trusted HTTPS</button><p>Restart required. Your sign-in and passkey address becomes the DuckDNS name on this Kinosail port. Sign in there with your password and MFA, then add your passkeys again for the new address.</p></fieldset></form></details></section><section><h2>Keep the private certificate</h2><p>No public DNS record is created. Devices will show a warning until they trust the <a href="/api/v1/agent-connections/certificate">Kinosail local CA certificate</a>.</p></section><section><h2>Remote access comes later</h2><p>For access away from home, Settings can guide you toward private Owner management through WireGuard or a separate Viewer-only public HTTPS listener after local setup is complete.</p></section><section class="wide" id="updates"><h2>Choose update checks</h2><p>Kinosail can ask GitHub for the latest public release. GitHub receives your public IP address, the request time, and a generic Kinosail user agent. Kinosail does not send its version, Server name, media details, profiles, device identifiers, cookies, or tokens.</p><p><strong class="status" role="status">{{.UpdateText}}</strong></p><form action="/onboarding/updates" method="post"><label>Update checks <select name="mode"><option value="manual" {{if not .Updates.Automatic}}selected{{end}}>Manual only</option><option value="automatic" {{if .Updates.Automatic}}selected{{end}}>Check about once a day</option></select></label><button>Save update choice</button></form><form action="/onboarding/updates/check" method="post"><button>Check GitHub now</button></form><p>Kinosail only shows an Owner notification. Installation stays manual and uses the verified installer.</p></section></div><p><a class="mode" href="/onboarding/household">Continue to household setup</a> <a class="mode" href="/onboarding/finish">Skip optional setup</a></p></main></body></html>`

const homeAssistantOnboardingHTML = `<section class="onboarding-optional"><span class="onboarding-optional-label">Optional connection</span><h2>Home Assistant</h2>{{template "configurationSource" .HomeAssistantControl}}<form action="/onboarding/home-assistant" method="post"><fieldset {{if .HomeAssistantControl.Managed}}disabled aria-disabled="true"{{end}}><label><input type="checkbox" name="enabled" value="true" {{if .HomeAssistant}}checked{{end}} data-home-assistant> Connect Home Assistant</label><p>{{if .HomeAssistant}}Local discovery and secure browser approval are ready. Turning this off withdraws discovery and revokes every connection.{{else}}Kinosail exposes no Home Assistant discovery, pairing, player, or media endpoints until you enable this.{{end}}</p><button>Save Home Assistant choice</button></fieldset></form>{{if .HomeAssistant}}<p><a class="button" href="https://my.home-assistant.io/redirect/config_flow_start/?domain=kinosail" target="_blank" rel="noopener">Add to Home Assistant</a></p><details><summary>Manual pairing</summary><p>Use this only when browser approval is unavailable.</p><form action="/settings/home-assistant/pair" method="post"><button class="quiet">Create one-time pairing code</button></form></details>{{end}}</section>`

const secureLocalOnboardingHTML = `<section class="wide onboarding-first-value"><span class="onboarding-ready-label">Ready on this Server</span><h2>Secure local access</h2><p><strong class="status">On by default.</strong> Kinosail uses HTTPS with a private certificate. It does not create public DNS records, open router ports, or relay media.</p><p>Browsers can continue after a certificate warning. Other devices can trust the <a href="/api/v1/agent-connections/certificate">Kinosail local CA certificate</a>.</p></section>`

const jellyfinOnboardingHTML = `<section class="wide onboarding-optional" id="jellyfin"><span class="onboarding-optional-label">Optional connection</span><h2>Jellyfin apps</h2>{{template "configurationSource" .JellyfinControl}}<p><strong>Trusted HTTPS is required.</strong> Many Jellyfin apps reject private certificates and cannot install a local certificate. Kinosail requires a supported trusted hostname so household members do not install a certificate or bypass a security warning.</p>{{if .Trusted.Configured}}<p role="status">Trusted HTTPS is saved. {{if .JellyfinCompatibility}}Jellyfin routes are available. Restart Kinosail before you connect an app.{{else}}You can now enable Jellyfin apps, then restart Kinosail once.{{end}}</p>{{else}}<p>Set up <a href="#trusted-https-configuration" data-open-disclosure="trusted-https-configuration">Trusted HTTPS</a> first. Jellyfin routes stay unavailable.</p>{{end}}<form action="/onboarding/jellyfin" method="post"><fieldset {{if or .JellyfinControl.Managed (not .Trusted.Configured)}}disabled aria-disabled="true"{{end}}><label><input type="checkbox" name="enabled" value="true" {{if .JellyfinCompatibility}}checked{{end}}> Allow compatible Jellyfin apps to connect</label><button>Save Jellyfin choice</button></fieldset></form></section>`

var tmdbOnboardingHTML = strings.NewReplacer(`action="/settings/configuration"`, `action="/onboarding/tmdb"`, `Restart Kinosail Server to start filling missing artwork automatically.`, `Restart Kinosail Server once after setup. Kinosail then fills missing artwork automatically.`, `class="wide integration-setup"`, `class="wide integration-setup onboarding-optional"`).Replace(tmdbConfigurationHTML)

var onboardingConnectionView = newLocalizedTemplate("onboarding-connection", onboardingConnectionTemplate())

func onboardingConnectionTemplate() string {
	content := strings.NewReplacer(
		`{{template "onboardingProgress"}}`, `{{template "onboardingProgress" .Step}}`,
		`<h1>Make every device feel at home.</h1><p>Choose the address your phones, TVs, and browsers will trust. Kinosail stays on your network unless you separately enable remote access.</p>`, `<h1>Connect the devices you already own.</h1><p>Local access is ready now. Add trusted HTTPS only if a phone, TV, or app needs it; you can change these choices later in Settings.</p>`,
		`<section class="wide"><h2>Trusted HTTPS with DuckDNS</h2>`, secureLocalOnboardingHTML+`<section class="wide onboarding-recommended" id="trusted-https"><h2>Trusted HTTPS for phones, TVs, and Jellyfin apps</h2>`,
		`<p>Use a free DuckDNS name and Let's Encrypt certificate`, `<p><strong>Required for Jellyfin apps. Recommended for phones and TVs.</strong> Use a free DuckDNS name and Let's Encrypt certificate`,
		`<details class="onboarding-disclosure" id="trusted-https-configuration">`, `<details class="onboarding-disclosure" id="trusted-https-configuration" {{if not .Trusted.Configured}}open{{end}}>`,
		`<section><h2>Keep the private certificate</h2><p>No public DNS record is created. Devices will show a warning until they trust the <a href="/api/v1/agent-connections/certificate">Kinosail local CA certificate</a>.</p></section>`, jellyfinOnboardingHTML,
		`<section><h2>Remote access comes later</h2>`, homeAssistantOnboardingHTML+`<section class="onboarding-optional"><span class="onboarding-optional-label">Optional connection</span><h2>Remote access comes later</h2>`,
		`<section class="wide" id="updates">`, tmdbOnboardingHTML+`<section class="wide" id="updates">`,
	).Replace(onboardingConnectionHTML)
	return ignoreNonPasswordSecretAutofill(settingControlHTML + `{{define "onboardingProgress"}}` + onboardingProgressHTML + `{{end}}` + providerNeutralTrustedHTTPSPage(updateChoicePage(content)))
}

func showOnboardingConnection(settings *settingsStore, updates *updateChecker, authURL string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		updateView := updates.View()
		configured := settings.configuration()
		data := struct {
			Trusted               trustedHTTPSView
			TrustedControl        settingControl
			TrustedRequirements   trustedHTTPSRequirements
			JellyfinCompatibility bool
			JellyfinControl       settingControl
			Updates               updateStatus
			UpdateText            string
			UpdateManagerText     string
			HomeAssistant         bool
			HomeAssistantControl  settingControl
			TMDB                  tmdbConfigurationView
			Step                  string
		}{settings.trustedHTTPS(), configurationControl(configured, "tls.duckdns"), trustedHTTPSRequirementView(configured), settings.jellyfinCompatibility(), configurationControl(configured, "integrations.jellyfin.enabled"), updateView, updateStatusText(updateView), updateManagerStatusText(updateView.Manager), settings.homeAssistant(), configurationControl(configured, "integrations.home_assistant.enabled"), settings.tmdbConfiguration(), "devices"}
		if data.JellyfinCompatibility {
			data.JellyfinControl.ConnectionURL = trustedConnectionURL(authURL, data.Trusted.Hostname)
		}
		if err := onboardingConnectionView.Execute(writer, request, data); err != nil {
			localizedError(writer, request, "connection onboarding failed", http.StatusInternalServerError)
		}
	}
}
