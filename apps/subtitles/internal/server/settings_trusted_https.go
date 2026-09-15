package server

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

var (
	errTrustedHTTPSConflict = errors.New("trusted HTTPS conflicts with the current deployment")
	errTrustedHTTPSStorage  = errors.New("trusted HTTPS configuration storage failed")
)

type trustedHTTPSInput struct {
	Provider, Domain, Token, Address string
	TermsAccepted                    bool
}

type trustedHTTPSView struct {
	Provider   string `json:"provider,omitempty"`
	Domain     string `json:"domain,omitempty"`
	Address    string `json:"address,omitempty"`
	Hostname   string `json:"hostname,omitempty"`
	Configured bool   `json:"configured"`
	TokenSet   bool   `json:"tokenConfigured"`
}

func (store *settingsStore) trustedHTTPS() trustedHTTPSView {
	store.mu.RLock()
	defer store.mu.RUnlock()
	config, err := trustedhttps.Parse(store.config.String("tls.duckdns"))
	if err != nil || config == (trustedhttps.Config{}) {
		return trustedHTTPSView{Provider: trustedhttps.ProviderDuckDNS}
	}
	return trustedHTTPSView{Provider: config.ProviderName(), Domain: config.Domain, Address: config.Address, Hostname: config.Hostname(), Configured: true, TokenSet: config.Token != ""}
}

func (store *settingsStore) setTrustedHTTPS(input trustedHTTPSInput) (trustedHTTPSView, error) { //nolint:cyclop,gocognit // Validation and conflict checks form one fail-closed settings transaction.
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.editableLocked("tls.duckdns"); err != nil {
		return trustedHTTPSView{}, err
	}
	if store.file == "" {
		return trustedHTTPSView{}, errTrustedHTTPSStorage
	}
	existing, _ := trustedhttps.Parse(store.config.String("tls.duckdns"))
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if provider == "" {
		provider = trustedhttps.ProviderDuckDNS
	}
	if input.Token == "" && provider == existing.ProviderName() {
		input.Token = existing.Token
	}
	config, err := trustedhttps.NewProviderConfig(provider, input.Domain, strings.TrimSpace(input.Token), input.Address, input.TermsAccepted)
	if err != nil {
		return trustedHTTPSView{}, err
	}
	raw, err := config.Encode()
	if err != nil {
		return trustedHTTPSView{}, err
	}
	if store.config.Source("tls.enabled") != "" && !store.config.Bool("tls.enabled") {
		return trustedHTTPSView{}, errors.Join(errTrustedHTTPSConflict, errors.New("HTTPS must be enabled"))
	}
	if store.config.String("remote.mode") == "https" {
		return trustedHTTPSView{}, errors.Join(errTrustedHTTPSConflict, errors.New("public HTTPS remote access is already enabled"))
	}
	if config.ProviderName() == trustedhttps.ProviderDuckDNS && store.config.String("remote.mode") != "off" && store.config.String("remote.mode") != "" && store.config.String("remote.duckdns_domain") == config.Domain {
		return trustedHTTPSView{}, errors.Join(errTrustedHTTPSConflict, errors.New("remote access must use a different DuckDNS subdomain"))
	}
	if listen := store.config.String("listen"); listen != "" {
		origin, originErr := configuration.TrustedOrigin(raw, listen)
		if originErr != nil {
			return trustedHTTPSView{}, originErr
		}
		if configured := store.config.String("auth.url"); configured != "" && configured != origin {
			return trustedHTTPSView{}, errors.Join(errTrustedHTTPSConflict, errors.New("the configured sign-in address does not match this trusted HTTPS address"))
		}
	}
	if err = configuration.Set(filepath.Dir(store.file), "tls.duckdns", raw); err != nil {
		slog.Error("save trusted HTTPS configuration failed", "error", err)
		return trustedHTTPSView{}, errTrustedHTTPSStorage
	}
	store.config.UpdateGUI("tls.duckdns", raw, false)
	return trustedHTTPSView{Provider: config.ProviderName(), Domain: config.Domain, Address: config.Address, Hostname: config.Hostname(), Configured: true, TokenSet: true}, nil
}

func (store *settingsStore) disableTrustedHTTPS() error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.editableLocked("tls.duckdns"); err != nil {
		return err
	}
	if store.file == "" {
		return errTrustedHTTPSStorage
	}
	if err := configuration.Delete(filepath.Dir(store.file), "tls.duckdns"); err != nil {
		slog.Error("disable trusted HTTPS configuration failed", "error", err)
		return errTrustedHTTPSStorage
	}
	store.config.UpdateGUI("tls.duckdns", "", true)
	return nil
}

func trustedHTTPSStatus(manager *trustedhttps.Manager) trustedhttps.Status {
	if manager == nil {
		return trustedhttps.Status{State: "disabled"}
	}
	return manager.Status()
}

func trustedConnectionURL(current, hostname string) string {
	parsed, err := url.Parse(current)
	if err != nil || hostname == "" {
		return current
	}
	host := hostname
	if port := parsed.Port(); port != "" {
		host = net.JoinHostPort(hostname, port)
	}
	return (&url.URL{Scheme: "https", Host: host}).String()
}

func apiTrustedHTTPS(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct {
			Provider, Domain, Token, Address string
			TermsAccepted                    bool `json:"termsAccepted"`
		}
		if !readJSON(writer, request, &input) {
			return
		}
		view, err := settings.setTrustedHTTPS(trustedHTTPSInput{input.Provider, input.Domain, input.Token, input.Address, input.TermsAccepted})
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, errManagedSetting) || errors.Is(err, errTrustedHTTPSConflict) {
				status = http.StatusConflict
			} else if errors.Is(err, errTrustedHTTPSStorage) {
				status = http.StatusInternalServerError
			}
			apiError(writer, err, status)
			return
		}
		writeJSON(writer, map[string]any{"status": "saved", "restartRequired": true, "trustedHttps": view}, http.StatusAccepted)
	}
}

func apiDisableTrustedHTTPS(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		if err := settings.disableTrustedHTTPS(); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, errManagedSetting) {
				status = http.StatusConflict
			}
			apiError(writer, err, status)
			return
		}
		writeJSON(writer, map[string]any{"status": "saved", "restartRequired": true}, http.StatusAccepted)
	}
}

func saveTrustedHTTPS(settings *settingsStore, redirect string) http.HandlerFunc {
	return trustedhttps.SettingsForm{
		Save: func(input trustedhttps.SettingsInput) error {
			_, err := settings.setTrustedHTTPS(trustedHTTPSInput(input))
			return err
		},
		Managed: errManagedSetting, Conflict: errTrustedHTTPSConflict, Storage: errTrustedHTTPSStorage,
		Error: localizedError, Redirect: redirect,
	}.ServeHTTP
}

func disableTrustedHTTPS(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := settings.disableTrustedHTTPS(); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, errManagedSetting) {
				status = http.StatusConflict
			}
			localizedError(writer, request, err.Error(), status)
			return
		}
		http.Redirect(writer, request, "/settings#trusted-https", http.StatusSeeOther)
	}
}

const trustedHTTPSProviderFields = `<label>DNS provider <select name="provider" required><option value="duckdns" {{if eq .Trusted.Provider "duckdns"}}selected{{end}}>DuckDNS (Easiest)</option><option value="desec" {{if eq .Trusted.Provider "desec"}}selected{{end}}>deSEC (More privacy)</option></select></label><p><strong>DuckDNS is the easiest default.</strong> It has the shortest setup. <strong>deSEC gives a little more privacy.</strong> It is nonprofit and open source, and it supports narrow API tokens that can limit Kinosail to this hostname and its certificate records.</p><p>Create a hostname at <a href="https://www.duckdns.org/" target="_blank" rel="noopener noreferrer">DuckDNS</a> or <a href="https://desec.io/" target="_blank" rel="noopener noreferrer">deSEC</a>. Then paste its details below.</p><details class="wizard-help" open><summary>Get the provider details</summary><ol><li><strong>Open a provider.</strong> Use <a href="https://www.duckdns.org/" target="_blank" rel="noopener noreferrer">DuckDNS</a> or <a href="https://desec.io/" target="_blank" rel="noopener noreferrer">deSEC</a>.</li><li><strong>Create one hostname.</strong> Use a new hostname for this Kinosail Subtitles Server.</li><li><strong>Create a narrow token.</strong> Limit it to this hostname when your provider supports token policies. Copy it once and keep it private.</li><li><strong>Return here.</strong> Choose the provider, enter the hostname, paste the token, and add the Server LAN address.</li></ol><p class="wizard-help-note">A provider token can change DNS records. Kinosail stores it in the protected secrets file and never shows it again.</p></details><label>Trusted hostname <input name="domain" value="{{.Trusted.Domain}}" placeholder="myhome-subtitles" maxlength="253" required></label><p>For DuckDNS, enter either a label or its full <code>duckdns.org</code> hostname. For deSEC, enter the full <code>dedyn.io</code> hostname.</p><p><strong>Example pair:</strong> use <code>myhome.duckdns.org</code> for Kinosail Player and <code>myhome-subtitles.duckdns.org</code> for Kinosail Subtitles. Both can use the same DNS account and LAN address.</p><p><strong>Network ports:</strong> no router port forwarding is needed. Allow the configured LAN port through the Server firewall for LAN devices only (default <code>38128</code>). Do not forward that port on your router. This does not make Kinosail Subtitles available away from home.</p><p><strong>You need:</strong> a DuckDNS or deSEC account, a hostname for this app, a provider token, the Server's private LAN IPv4 address, outbound internet access, and agreement to the Let's Encrypt terms.</p><details><summary>How trusted HTTPS works</summary><ol><li>Kinosail points the hostname at the private LAN address.</li><li>It creates a temporary DNS TXT record so Let's Encrypt can verify the hostname without connecting through your router.</li><li>It downloads the certificate, removes the TXT record, and renews the certificate automatically.</li><li>After restart, Kinosail serves the certificate on this app's LAN port.</li></ol></details><label>Provider token <input type="password" name="token" autocomplete="off" placeholder="{{if .Trusted.TokenSet}}Leave blank to keep the current token{{else}}Paste your provider token{{end}}" maxlength="512" {{if not .Trusted.TokenSet}}required{{end}}></label>`

func providerNeutralTrustedHTTPSPage(page string) string {
	legacyFields := `<label>DuckDNS subdomain <span><input name="domain" value="{{.Trusted.Domain}}" placeholder="my-kinosail" maxlength="63" required>.duckdns.org</span></label><label>DuckDNS token <input type="password" name="token" autocomplete="off" placeholder="{{if .Trusted.TokenSet}}Leave blank to keep the current token{{else}}Paste your DuckDNS token{{end}}" maxlength="128" {{if not .Trusted.TokenSet}}required{{end}}></label>`
	return modernTrustedHTTPSProviderChoice(strings.NewReplacer(
		`<h2>Trusted HTTPS with DuckDNS`, `<h2>Trusted HTTPS`,
		`or use DuckDNS below`, `or use trusted HTTPS below`,
		`set up DuckDNS HTTPS`, `set up trusted HTTPS`,
		`Use a free DuckDNS name and Let's Encrypt certificate`, `Use a supported DNS provider and Let's Encrypt certificate`,
		`<p>Kinosail stays on your LAN. This option does not open a router port or relay media. First create a subdomain at <a href="https://www.duckdns.org/" rel="noreferrer">DuckDNS</a>, then paste its details below.</p>`, `<p>Kinosail stays on your LAN. This option does not open a router port or relay media.</p>`,
		`The DuckDNS token can update every hostname in that account; Kinosail stores it in the protected secrets file and never shows it here.`, `Use the narrowest token your provider supports. Kinosail stores it in the protected secrets file and never shows it here. No certificate install is required.`,
		legacyFields, trustedHTTPSProviderFields,
		`Your sign-in and passkey address becomes the DuckDNS name`, `Your sign-in and passkey address becomes the trusted hostname`,
	).Replace(page))
}

func modernTrustedHTTPSProviderChoice(page string) string {
	return strings.ReplaceAll(page, `<label>DNS provider <select name="provider" required><option value="duckdns" {{if eq .Trusted.Provider "duckdns"}}selected{{end}}>DuckDNS (Easiest)</option><option value="desec" {{if eq .Trusted.Provider "desec"}}selected{{end}}>deSEC (More privacy)</option></select></label>`, `<fieldset class="choice-set choice-cards"><legend>DNS provider</legend><label class="choice-option"><input type="radio" name="provider" value="duckdns" {{if ne .Trusted.Provider "desec"}}checked{{end}}><span><strong>DuckDNS · Easiest</strong><small>Shortest setup for a trusted LAN name.</small></span></label><label class="choice-option"><input type="radio" name="provider" value="desec" {{if eq .Trusted.Provider "desec"}}checked{{end}}><span><strong>deSEC · More privacy</strong><small>Supports narrow tokens limited to this hostname.</small></span></label></fieldset>`)
}
