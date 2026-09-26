package server

import (
	"net/http"
	"slices"
	"strings"

	"github.com/MikeO7/kinosail-subtitles/internal/subtitlelanguage"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

const subtitleSettingsHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><meta name="theme-color" content="#0b0d0b"><title>Settings · Kinosail Subtitles</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"></head><body class="settings-page subtitle-settings"><a class="skip" href="#main">Skip to content</a><main id="main" class="settings-shell"><a class="back" href="/">{{icon "back"}} Subtitle overview</a><header class="settings-intro"><h1>Subtitle settings</h1><p>Choose where Kinosail looks, which languages each title needs, and how often it checks for changes.</p></header><nav class="settings-nav" aria-label="Settings sections"><a href="#language">Languages</a><a href="#provider">Provider</a><a href="#libraries">Libraries</a><a href="#automation">Automation</a><a href="#appearance">Appearance</a><a href="#trusted-https">Access</a><a href="#account">Account</a></nav><div class="settings-flow">
<section id="language"><h2>Preferred languages</h2><p>A media file is ready when it has text subtitles in every selected language. The first language is your primary choice.</p><ol class="subtitle-language-list" aria-label="Preferred subtitle languages">{{range .SelectedLanguages}}<li><span><strong>{{.Name}}</strong><small><code>{{.Tag}}</code> · {{.Support}}{{if .Primary}} · Primary{{end}}</small></span>{{if not $.LanguageManaged}}<form action="/settings/subtitles" method="post"><input type="hidden" name="language" value="{{.Tag}}"><button class="quiet" name="action" value="earlier" aria-label="Move {{.Name}} earlier" {{if not .CanMoveEarlier}}disabled{{end}}>↑</button><button class="quiet" name="action" value="later" aria-label="Move {{.Name}} later" {{if not .CanMoveLater}}disabled{{end}}>↓</button><button class="quiet" name="action" value="remove" aria-label="Remove {{.Name}}" {{if .Only}}disabled{{end}}>Remove</button></form>{{end}}</li>{{end}}</ol>{{if .LanguageManaged}}<p class="status">Managed by deployment configuration.</p>{{else if .AvailableLanguages}}<form action="/settings/subtitles" method="post" class="subtitle-language-add"><label>Add a language<select name="language" required>{{range .AvailableLanguages}}<option value="{{.Tag}}">{{.Name}} ({{.Tag}}) · {{.Support}}</option>{{end}}</select></label><button name="action" value="add">Add language</button></form>{{else}}<form class="subtitle-language-add" aria-describedby="language-limit"><label>Add a language<select disabled><option>20-language limit reached</option></select></label><button disabled>Add language</button></form><p class="status" id="language-limit">You have selected 20 languages. Remove one before adding another.</p>{{end}}{{if not .LanguageManaged}}<form action="/settings/subtitles" method="post"><input type="hidden" name="language" value="{{.Language}}"><label>Preferred subtitle role<select name="preference"><option value="standard" {{if eq .Preference "standard"}}selected{{end}}>Standard dialogue</option><option value="sdh" {{if eq .Preference "sdh"}}selected{{end}}>SDH and captions</option></select></label><button>Save subtitle role</button></form>{{end}}</section>
<section id="provider"><h2>Subtitle providers</h2><p><strong class="status">{{.Provider}}</strong></p><p>One provider is enough. Kinosail searches every configured provider automatically and selects one trusted match.</p><div class="provider-health-list">{{range .Providers}}<p><strong>{{.Name}}</strong> · <span class="subtitle-state {{if eq .State "connected"}}ready{{end}}">{{.State}}</span>{{if .Remaining}} · {{.Remaining}} remaining{{end}}{{if .NextRetry}} · retry {{.NextRetry}}{{end}}{{if .LastSafeError}} · {{.LastSafeError}}{{end}}</p>{{end}}</div><p>Need an account? {{range .ProviderSetup}}<a class="mode" href="{{.AccountURL}}" target="_blank" rel="noopener noreferrer">Create {{.Name}} account</a> {{end}}</p><p>On SubSource, choose Create Account.</p><form action="/subtitles/providers/test" method="post"><button>Test provider credentials</button></form><p>Embedded tracks stay local. Provider searches receive the filename, language, media identity, and a non-cryptographic file hash when supported.</p><p><a class="button" href="/settings/configuration#integrations.subdl.api_key">Configure SubDL</a> <a class="mode" href="/settings/configuration#integrations.opensubtitles.api_key">Configure OpenSubtitles</a></p><h3>SubSource</h3>{{template "configurationSource" .SubSourceControl}}{{if .SubSourceConfigured}}<p>A SubSource key and personal-use acceptance are saved.</p>{{else}}<p>SubSource adds broad free coverage. Its files remain unchanged and stay inside your household.</p>{{end}}{{if not .SubSourceControl.Managed}}<form action="/settings/subtitles/subsource" method="post"><label>SubSource API key<input name="apiKey" type="password" autocomplete="new-password" maxlength="4096" placeholder="{{if .SubSourceConfigured}}Leave blank to keep the configured key{{else}}Paste your SubSource API key{{end}}" {{if not .SubSourceConfigured}}required{{end}}></label><label><input type="checkbox" name="personalUse" value="true" required> Use SubSource only for personal household use and accept its terms</label><button>Save SubSource</button><p>Changes take effect immediately.</p></form>{{if .SubSourceConfigured}}<form action="/settings/subtitles/subsource/reset" method="post"><button class="quiet">Disable SubSource</button></form>{{end}}{{end}}</section>
<section class="wide" id="libraries"><h2>Media Libraries</h2><p>Media mount: <code>{{.MediaRoot}}</code></p><p>Give the Kinosail container permission to save subtitle files in these folders.</p>{{range .Libraries}}<form action="/settings/libraries/remove" method="post"><fieldset {{if $.LibrariesManaged}}disabled aria-disabled="true"{{end}}><code>{{.}}</code> <button class="quiet" name="path" value="{{.}}">Remove</button></fieldset></form>{{end}}<form action="/settings/libraries" method="post"><fieldset {{if .LibrariesManaged}}disabled aria-disabled="true"{{end}}><label>Folder inside media mount<input name="path" placeholder="Movies" maxlength="4096" required></label><button>Add library</button></fieldset></form>{{if .LibrariesManaged}}<p class="status">Managed by deployment configuration.</p>{{end}}</section>
<section id="automation"><h2>Library monitoring</h2><p>Kinosail detects new files when copying finishes. Scheduled scans find missing subtitles and replace automatically managed subtitles when a better trusted match is available.</p><form action="/settings/scans" method="post"><fieldset {{if .ScansManaged}}disabled aria-disabled="true"{{end}}><label>Safety scan<select name="frequency"><option value="default" {{if eq .Frequency "default"}}selected{{end}}>Server default</option><option value="5m" {{if eq .Frequency "5m"}}selected{{end}}>Every 5 minutes</option><option value="15m" {{if eq .Frequency "15m"}}selected{{end}}>Every 15 minutes</option><option value="1h" {{if eq .Frequency "1h"}}selected{{end}}>Every hour</option><option value="off" {{if eq .Frequency "off"}}selected{{end}}>Only filesystem events</option></select></label><button>Save schedule</button></fieldset></form>{{if .ScansManaged}}<p class="status">Managed by deployment configuration.</p>{{end}}</section>
<section id="appearance"><h2>Appearance</h2><p>Match your device’s appearance or choose light or dark.</p><label>Theme <select aria-label="Theme" data-theme-choice><option value="dark">Dark (default)</option><option value="light">Light</option><option value="system">System</option></select></label></section>
<section class="wide" id="trusted-https"><h2>Trusted HTTPS with DuckDNS (Recommended)</h2>{{template "configurationSource" .TrustedControl}}<p><strong>Recommended for phones and browsers. Optional.</strong> Use a free DuckDNS name and Let's Encrypt certificate when a device rejects the private certificate.</p><p>Status: <strong class="status">{{if .Trusted.Configured}}{{if eq .TrustedStatus.State "disabled"}}Saved — restart required{{else}}{{.TrustedStatus.State}}{{end}}{{else}}Not configured{{end}}</strong>{{if .Trusted.Hostname}} · <code>{{.Trusted.Hostname}}</code>{{end}}</p>{{if .TrustedStatus.CertificateExpires}}<p>Certificate expires {{.TrustedStatus.CertificateExpires}}.</p>{{end}}{{if .TrustedStatus.Error}}<p role="alert">{{.TrustedStatus.Error}}</p>{{end}}<p>Kinosail stays on your LAN. This option does not open a router port or relay media. First create a subdomain at <a href="https://www.duckdns.org/" rel="noreferrer">DuckDNS</a>, then paste its details below.</p><p>Privacy note: the private LAN address is published in public DNS and the hostname appears in public certificate-transparency logs. The DuckDNS token can update every hostname in that account; Kinosail stores it in the protected secrets file and never shows it here.</p><form action="/settings/trusted-https" method="post"><fieldset {{if .TrustedControl.Managed}}disabled aria-disabled="true"{{end}}><label>DuckDNS subdomain <span><input name="domain" value="{{.Trusted.Domain}}" placeholder="my-kinosail" maxlength="63" required>.duckdns.org</span></label><label>DuckDNS token <input type="password" name="token" autocomplete="off" placeholder="{{if .Trusted.TokenSet}}Leave blank to keep the current token{{else}}Paste your DuckDNS token{{end}}" maxlength="128" {{if not .Trusted.TokenSet}}required{{end}}></label><label>Kinosail LAN address <input name="address" inputmode="decimal" value="{{.Trusted.Address}}" placeholder="192.168.1.10" maxlength="15" required></label><label><input type="checkbox" name="termsAccepted" value="true" required> Allow DNS validation and accept the Let's Encrypt subscriber agreement</label><button>Save trusted HTTPS</button><p>Restart required. Your sign-in and passkey address becomes the DuckDNS name on this Kinosail port. Sign in there with your password and MFA, then add your passkeys again for the new address.</p></fieldset></form>{{if and .Trusted.Configured (not .TrustedControl.Managed)}}<form action="/settings/trusted-https/disable" method="post"><button class="quiet">Disable trusted HTTPS after restart</button></form>{{end}}</section>
<section id="account"><h2>Owner and recovery</h2><p>Manage passkeys, authenticators, sessions, encrypted backups, and advanced deployment values.</p><p><a class="mode" href="/account">Owner account</a> <a class="mode" href="/settings/backups">Backups</a> <a class="mode" href="/settings/configuration">Advanced configuration</a></p><form action="/settings/onboarding" method="post"><button class="quiet">Open setup guide</button></form></section>
</div></main></body></html>`

const subtitleCleanupSettingsHTML = `<section id="cleanup"><h2>Delete subtitle languages</h2><p>Optional. Cleanup is off until you enable it for this preview. Select one or more languages to keep; confirming deletion also makes them your preferred languages. Only tagged .srt and .vtt files are eligible. Embedded tracks and files with uncertain language stay in place.</p><form action="/settings/subtitles/cleanup" method="get"><label><input type="checkbox" name="enabled" value="on" required> Enable subtitle language cleanup</label><label>Languages to keep<select name="language" multiple size="6" required aria-describedby="cleanup-language-help">{{range .AllLanguages}}<option value="{{.Tag}}" {{if .Selected}}selected{{end}}>{{.Name}} ({{.Tag}})</option>{{end}}</select></label><p id="cleanup-language-help">Select at least one language. On a keyboard, hold Command or Ctrl to select several.</p><label>Forced subtitles in kept languages<select name="forced"><option value="keep">Keep</option><option value="delete">Delete</option></select></label><button>Preview files to delete</button></form></section>`

var subtitleSettingsView = newLocalizedTemplate("subtitle-settings", ignoreNonPasswordSecretAutofill(settingControlHTML+providerNeutralTrustedHTTPSPage(updateChoicePage(strings.ReplaceAll(strings.ReplaceAll(subtitleSettingsHTML, `<a href="#provider">Provider</a>`, `<a href="#cleanup">Cleanup</a><a href="#provider">Provider</a>`), `<section id="provider">`, subtitleCleanupSettingsHTML+`<section id="provider">`)))))

type subtitleSettingsData struct {
	MediaRoot, Language, Preference, Frequency, Provider string
	Libraries                                            []string
	Providers                                            []subtitleProviderHealth
	ProviderSetup                                        []subtitleProviderSetup
	SelectedLanguages                                    []subtitleLanguageView
	AvailableLanguages                                   []subtitleLanguageOption
	AllLanguages                                         []subtitleLanguageOption
	LanguageManaged, LibrariesManaged, ScansManaged      bool
	ProviderConfigured                                   bool
	Trusted                                              trustedHTTPSView
	TrustedControl                                       settingControl
	SubSourceControl                                     settingControl
	SubSourceConfigured                                  bool
	TrustedStatus                                        trustedhttps.Status
}

type subtitleProviderSetup struct {
	Name, Instructions, AccountURL, DocsURL, SettingsURL string
	Configured                                           bool
}

type subtitleLanguageView struct {
	Tag, Name, Support           string
	Primary, Only                bool
	CanMoveEarlier, CanMoveLater bool
}

type subtitleLanguageOption struct {
	Tag, Name, Support string
	Selected           bool
}

func subtitleSettingsPageData(settings *settingsStore, provider *subtitleProvider, manager *trustedhttps.Manager) subtitleSettingsData {
	value := settings.snapshot()
	configured := settings.configuration()
	languages := settings.subtitleLanguages()
	selected, available, all := subtitleLanguageViews(languages)
	data := subtitleSettingsData{
		MediaRoot: settings.mediaRoot, Language: languages[0], Preference: settings.subtitlePreference(), Frequency: settings.scanFrequency(), Provider: provider.label(), Libraries: value.Libraries, Providers: provider.healthViews(),
		SelectedLanguages: selected, AvailableLanguages: available,
		AllLanguages:        all,
		LanguageManaged:     configurationControl(configured, "subtitles.language").Managed,
		LibrariesManaged:    configurationControl(configured, "libraries").Managed,
		ScansManaged:        configurationControl(configured, "scanning.frequency").Managed,
		ProviderConfigured:  provider.configured(),
		SubSourceControl:    configurationControl(configured, subSourceConfigurationKeys...),
		SubSourceConfigured: configured.Public("integrations.subsource.api_key").Configured,
	}
	data.ProviderSetup = subtitleProviderSetupViews(data.Providers)
	if manager != nil {
		data.Trusted = settings.trustedHTTPS()
		data.TrustedControl = configurationControl(configured, "tls.duckdns")
		data.TrustedStatus = trustedHTTPSStatus(manager)
	}
	return data
}

func subtitleProviderSetupViews(health []subtitleProviderHealth) []subtitleProviderSetup {
	setup := map[string]subtitleProviderSetup{
		"SubDL": {
			Name: "SubDL", Instructions: "Create a free account, open its API panel, and generate a Search and Download API key.",
			AccountURL: "https://subdl.com/panel/register", DocsURL: "https://subdl.com/api-doc", SettingsURL: "/settings/configuration#integrations.subdl.api_key",
		},
		"OpenSubtitles": {
			Name: "OpenSubtitles", Instructions: "Create or sign in to an OpenSubtitles.com account, then create an API key in your account. Kinosail also needs that account's username and password.",
			AccountURL: "https://www.opensubtitles.com/en/users/sign_up", DocsURL: "https://opensubtitles.stoplight.io/docs/opensubtitles-api/e3750fd63a100-getting-started", SettingsURL: "/settings/configuration#integrations.opensubtitles",
		},
		"SubSource": {
			Name: "SubSource", Instructions: "On SubSource, choose Create Account. Then open My Profile and generate an API key. Kinosail requires personal-use acceptance when you save it.",
			AccountURL: "https://subsource.net/", DocsURL: "https://subsource.net/api-docs", SettingsURL: "/settings#provider",
		},
	}
	result := make([]subtitleProviderSetup, 0, len(health))
	for _, provider := range health {
		view, ok := setup[provider.Name]
		if !ok {
			continue
		}
		view.Configured = provider.Configured
		result = append(result, view)
	}
	return result
}

func subtitleLanguageViews(selected []string) ([]subtitleLanguageView, []subtitleLanguageOption, []subtitleLanguageOption) {
	views := make([]subtitleLanguageView, 0, len(selected))
	available := make([]subtitleLanguageOption, 0, len(subtitlelanguage.Catalog())-len(selected))
	all := make([]subtitleLanguageOption, 0, len(subtitlelanguage.Catalog()))
	for _, choice := range subtitlelanguage.Catalog() {
		support := subtitleLanguageSupport(choice)
		index := slices.Index(selected, choice.Tag)
		all = append(all, subtitleLanguageOption{choice.Tag, choice.Name, support, index >= 0})
		if index < 0 {
			if slices.ContainsFunc(selected, func(language string) bool { return subtitleLanguagesOverlap(language, choice.Tag) }) {
				continue
			}
			available = append(available, subtitleLanguageOption{Tag: choice.Tag, Name: choice.Name, Support: support})
			continue
		}
		views = append(views, subtitleLanguageView{Tag: choice.Tag, Name: choice.Name, Support: support, Primary: index == 0, Only: len(selected) == 1, CanMoveEarlier: index > 0, CanMoveLater: index < len(selected)-1})
	}
	slices.SortStableFunc(views, func(left, right subtitleLanguageView) int {
		return slices.Index(selected, left.Tag) - slices.Index(selected, right.Tag)
	})
	slices.SortStableFunc(all, func(left, right subtitleLanguageOption) int {
		leftIndex, rightIndex := slices.Index(selected, left.Tag), slices.Index(selected, right.Tag)
		if leftIndex < 0 {
			leftIndex = len(selected)
		}
		if rightIndex < 0 {
			rightIndex = len(selected)
		}
		return leftIndex - rightIndex
	})
	if len(selected) == maximumSubtitleLanguages {
		available = nil
	}
	return views, available, all
}

func subtitleLanguageSupport(choice subtitlelanguage.Choice) string {
	providers := make([]string, 0, 3)
	for _, provider := range []struct {
		id   subtitlelanguage.Provider
		name string
	}{{subtitlelanguage.SubDL, "SubDL"}, {subtitlelanguage.OpenSubtitles, "OpenSubtitles"}, {subtitlelanguage.SubSource, "SubSource"}} {
		if choice.Providers[provider.id] {
			providers = append(providers, provider.name)
		}
	}
	if len(providers) == 0 {
		return "No configured provider"
	}
	return strings.Join(providers, ", ")
}

func showSubtitleSettings(settings *settingsStore, provider *subtitleProvider, manager *trustedhttps.Manager) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := subtitleSettingsView.Execute(writer, request, subtitleSettingsPageData(settings, provider, manager)); err != nil {
			localizedError(writer, request, "subtitle settings are unavailable", http.StatusInternalServerError)
		}
	}
}
