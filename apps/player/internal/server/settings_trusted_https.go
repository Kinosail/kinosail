package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

var (
	errTrustedHTTPSConflict = errors.New("trusted HTTPS conflicts with the current deployment")
	errTrustedHTTPSCheck    = errors.New("trusted HTTPS connection test failed")
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

type trustedHTTPSRequirements struct {
	Present, HTTPSDisabled, PublicRemote bool
	SignInAddress, ReservedHostname      string
}

func trustedHTTPSRequirementView(configured configuration.Snapshot) trustedHTTPSRequirements {
	view := trustedHTTPSRequirements{
		HTTPSDisabled: configured.Source("tls.enabled") != "" && !configured.Bool("tls.enabled"),
		PublicRemote:  configured.String("remote.mode") == "https",
		SignInAddress: configured.String("auth.url"),
	}
	if domain := configured.String("remote.duckdns_domain"); domain != "" && configured.String("remote.mode") != "off" && !view.PublicRemote {
		view.ReservedHostname = domain + ".duckdns.org"
	}
	view.Present = view.HTTPSDisabled || view.PublicRemote || view.SignInAddress != "" || view.ReservedHostname != ""
	return view
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
	config, raw, err := store.validTrustedHTTPSLocked(input)
	if err != nil {
		return trustedHTTPSView{}, err
	}
	if err = configuration.Set(filepath.Dir(store.file), "tls.duckdns", raw); err != nil {
		slog.Error("save trusted HTTPS configuration failed", "error", err)
		return trustedHTTPSView{}, errTrustedHTTPSStorage
	}
	store.config.UpdateGUI("tls.duckdns", raw, false)
	return trustedHTTPSConfigView(config), nil
}

func (store *settingsStore) testTrustedHTTPS(ctx context.Context, input trustedHTTPSInput) (trustedHTTPSView, error) {
	config, err := store.validateTrustedHTTPS(input)
	if err != nil {
		return trustedHTTPSView{}, err
	}
	if err = store.trustedHTTPSCheck(ctx, config); err != nil {
		return trustedHTTPSView{}, fmt.Errorf("%w: %s", errTrustedHTTPSCheck, err.Error())
	}
	return trustedHTTPSConfigView(config), nil
}

func (store *settingsStore) validateTrustedHTTPS(input trustedHTTPSInput) (trustedhttps.Config, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.editableLocked("tls.duckdns"); err != nil {
		return trustedhttps.Config{}, err
	}
	config, _, err := store.validTrustedHTTPSLocked(input)
	return config, err
}

func (store *settingsStore) validTrustedHTTPSLocked(input trustedHTTPSInput) (trustedhttps.Config, string, error) { //nolint:cyclop // All semantic checks must pass before either storage or a DNS update.
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
		return trustedhttps.Config{}, "", err
	}
	raw, err := config.Encode()
	if err != nil {
		return trustedhttps.Config{}, "", err
	}
	if store.config.Source("tls.enabled") != "" && !store.config.Bool("tls.enabled") {
		return trustedhttps.Config{}, "", errors.Join(errTrustedHTTPSConflict, errors.New("HTTPS must be enabled"))
	}
	if store.config.String("remote.mode") == "https" {
		return trustedhttps.Config{}, "", errors.Join(errTrustedHTTPSConflict, errors.New("public HTTPS remote access is already enabled"))
	}
	if config.ProviderName() == trustedhttps.ProviderDuckDNS && store.config.String("remote.mode") != "off" && store.config.String("remote.mode") != "" && store.config.String("remote.duckdns_domain") == config.Domain {
		return trustedhttps.Config{}, "", errors.Join(errTrustedHTTPSConflict, errors.New("remote access must use a different DuckDNS subdomain"))
	}
	listen := store.config.String("listen")
	if listen == "" {
		return config, raw, nil
	}
	origin, originErr := configuration.TrustedOrigin(raw, listen)
	if originErr != nil {
		return trustedhttps.Config{}, "", originErr
	}
	if configured := store.config.String("auth.url"); configured != "" && configured != origin {
		return trustedhttps.Config{}, "", errors.Join(errTrustedHTTPSConflict, errors.New("the configured sign-in address does not match this trusted HTTPS address"))
	}
	return config, raw, nil
}

func trustedHTTPSConfigView(config trustedhttps.Config) trustedHTTPSView {
	return trustedHTTPSView{Provider: config.ProviderName(), Domain: config.Domain, Address: config.Address, Hostname: config.Hostname(), Configured: true, TokenSet: true}
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

const trustedHTTPSProviderFields = `{{if .TrustedRequirements.Present}}<div class="trusted-https-requirements" role="note"><strong>Before entering a token</strong><ul>{{if .TrustedRequirements.HTTPSDisabled}}<li>HTTPS is disabled by deployment. Enable HTTPS before saving trusted HTTPS.</li>{{end}}{{if .TrustedRequirements.PublicRemote}}<li>Public HTTPS remote access is already enabled. Turn it off before configuring trusted LAN HTTPS.</li>{{end}}{{if .TrustedRequirements.SignInAddress}}<li>The trusted hostname and port must match the configured sign-in address: <code>{{.TrustedRequirements.SignInAddress}}</code>. Update <a href="/settings/configuration#auth.url">Sign-in address</a> before using a different hostname.</li>{{end}}{{if .TrustedRequirements.ReservedHostname}}<li>Remote access already uses <code>{{.TrustedRequirements.ReservedHostname}}</code>. Choose a different trusted hostname.</li>{{end}}</ul></div>{{end}}<label>DNS provider <select name="provider" required><option value="duckdns" {{if eq .Trusted.Provider "duckdns"}}selected{{end}}>DuckDNS (Easiest)</option><option value="desec" {{if eq .Trusted.Provider "desec"}}selected{{end}}>deSEC (More privacy)</option></select></label><p><strong>DuckDNS is the easiest default.</strong> It has the shortest setup. <strong>deSEC gives a little more privacy.</strong> It is nonprofit and open source, and it supports narrow API tokens that can limit Kinosail to this hostname and its certificate records.</p><p>Create a hostname at <a href="https://www.duckdns.org/" target="_blank" rel="noopener noreferrer">DuckDNS</a> or <a href="https://desec.io/" target="_blank" rel="noopener noreferrer">deSEC</a>. Then paste its details below.</p><label>Trusted hostname <input name="domain" value="{{.Trusted.Domain}}" placeholder="myhome" maxlength="253" required></label><p>For DuckDNS, enter either a label or its full <code>duckdns.org</code> hostname. For deSEC, enter the full <code>dedyn.io</code> hostname.</p><p><strong>Example pair:</strong> use <code>myhome.duckdns.org</code> for Kinosail Player and <code>myhome-subtitles.duckdns.org</code> for Kinosail Subtitles. Both can use the same DNS account and LAN address.</p><p><strong>Network ports:</strong> no router port forwarding is needed. Allow the configured LAN port through the Server firewall for LAN devices only (default <code>38127</code>). Do not forward that port on your router. Public remote access is a separate feature and normally uses TCP <code>443</code>.</p><p><strong>You need:</strong> a DuckDNS or deSEC account, a hostname for this app, a provider token, the Server's private LAN IPv4 address, outbound internet access, and agreement to the Let's Encrypt terms.</p><details><summary>How trusted HTTPS works</summary><ol><li>Kinosail points the hostname at the private LAN address.</li><li>It creates a temporary DNS TXT record so Let's Encrypt can verify the hostname without connecting through your router.</li><li>It downloads the certificate, removes the TXT record, and renews the certificate automatically.</li><li>After restart, Kinosail serves the certificate on this app's LAN port.</li></ol></details><label>Provider token <input type="password" name="token" autocomplete="off" placeholder="{{if .Trusted.TokenSet}}Leave blank to keep the current token{{else}}Paste your provider token{{end}}" maxlength="512" {{if not .Trusted.TokenSet}}required{{end}}></label>`

const trustedHTTPSTestFields = `<div class="trusted-https-test"><p>Test updates this hostname to the LAN address above. It verifies the provider token but does not save these settings.</p><div><button class="quiet" type="button" data-trusted-https-test>Test DNS connection</button><output aria-live="polite" data-trusted-https-test-status></output></div></div>`

func providerNeutralTrustedHTTPSPage(page string) string {
	legacyFields := `<label>DuckDNS subdomain <span><input name="domain" value="{{.Trusted.Domain}}" placeholder="my-kinosail" maxlength="63" required>.duckdns.org</span></label><label>DuckDNS token <input type="password" name="token" autocomplete="off" placeholder="{{if .Trusted.TokenSet}}Leave blank to keep the current token{{else}}Paste your DuckDNS token{{end}}" maxlength="128" {{if not .Trusted.TokenSet}}required{{end}}></label>`
	return modernTrustedHTTPSProviderChoice(trustedHTTPSAddressField(strings.NewReplacer(
		`<h2>Trusted HTTPS with DuckDNS`, `<h2>Trusted HTTPS`,
		`or use DuckDNS below`, `or use trusted HTTPS below`,
		`set up DuckDNS HTTPS`, `set up trusted HTTPS`,
		`Use a free DuckDNS name and Let's Encrypt certificate`, `Use a supported DNS provider and Let's Encrypt certificate`,
		`<p>Kinosail stays on your LAN. This option does not open a router port or relay media. First create a subdomain at <a href="https://www.duckdns.org/" rel="noreferrer">DuckDNS</a>, then paste its details below.</p>`, `<p>Kinosail stays on your LAN. This option does not open a router port or relay media.</p>`,
		`The DuckDNS token can update every hostname in that account; Kinosail stores it in the protected secrets file and never shows it here.`, `Use the narrowest token your provider supports. Kinosail stores it in the protected secrets file and never shows it here. No certificate install is required.`,
		legacyFields, trustedHTTPSProviderFields,
		`Your sign-in and passkey address becomes the DuckDNS name`, `Your sign-in and passkey address becomes the trusted hostname`,
		`<label><input type="checkbox" name="termsAccepted" value="true" required> Allow DNS validation and accept the Let's Encrypt subscriber agreement</label>`, `<label><input type="checkbox" name="termsAccepted" value="true" required> Allow DNS validation and accept the Let's Encrypt subscriber agreement</label>`+trustedHTTPSTestFields,
	).Replace(page)))
}

func modernTrustedHTTPSProviderChoice(page string) string {
	return strings.ReplaceAll(page, `<label>DNS provider <select name="provider" required><option value="duckdns" {{if eq .Trusted.Provider "duckdns"}}selected{{end}}>DuckDNS (Easiest)</option><option value="desec" {{if eq .Trusted.Provider "desec"}}selected{{end}}>deSEC (More privacy)</option></select></label>`, `<fieldset class="choice-set choice-cards"><legend>DNS provider</legend><label class="choice-option"><input type="radio" name="provider" value="duckdns" {{if ne .Trusted.Provider "desec"}}checked{{end}}><span><strong>DuckDNS · Easiest</strong><small>Shortest setup for a trusted LAN name.</small></span></label><label class="choice-option"><input type="radio" name="provider" value="desec" {{if eq .Trusted.Provider "desec"}}checked{{end}}><span><strong>deSEC · More privacy</strong><small>Supports narrow tokens limited to this hostname.</small></span></label></fieldset>`)
}

func trustedHTTPSAddressField(page string) string {
	return strings.ReplaceAll(page,
		`<label>Kinosail LAN address <input name="address" inputmode="decimal" value="{{.Trusted.Address}}" placeholder="192.168.1.10" maxlength="15" required></label>`,
		`<label>Kinosail LAN address <input name="address" inputmode="text" autocapitalize="none" spellcheck="false" value="{{.Trusted.Address}}" placeholder="192.168.1.10 or server.nox" maxlength="253" required></label><p>Enter a private IPv4 address or a local hostname such as <code>server.nox</code>. Kinosail resolves local hostnames before updating DuckDNS.</p>`)
}
