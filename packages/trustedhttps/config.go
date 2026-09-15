// Package trustedhttps provides optional public trust for a LAN-only Kinosail address.
package trustedhttps

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/netip"
	"strings"
)

const (
	ProviderDeSEC   = "desec"
	ProviderDuckDNS = "duckdns"
	duckDNSSuffix   = ".duckdns.org"
)

// Config stores one complete trusted HTTPS setup as an atomic secret document.
type Config struct {
	Provider string `json:"provider,omitempty"`
	Domain   string `json:"domain"`
	Token    string `json:"token"`
	Address  string `json:"address"`
	Terms    bool   `json:"termsAccepted"`
}

// NewProviderConfig normalizes owner input and validates the complete trust boundary.
func NewProviderConfig(provider, hostname, token, address string, terms bool) (Config, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = ProviderDuckDNS
	}
	domain := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(hostname), "."))
	if provider == ProviderDuckDNS {
		domain = strings.TrimSuffix(domain, duckDNSSuffix)
	}
	config := Config{Provider: provider, Domain: domain, Token: strings.TrimSpace(token), Address: strings.TrimSpace(address), Terms: terms}
	return config, config.Validate()
}

// Parse strictly decodes one bounded stored configuration document.
func Parse(raw string) (Config, error) { //nolint:cyclop,gocognit,funlen // Strict decoding and canonical validation share one persisted-input boundary.
	if raw == "" {
		return Config{}, nil
	}
	if len(raw) > 2048 {
		return Config{}, errors.New("trusted HTTPS configuration is too large")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	var config Config
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return Config{}, errors.New("trusted HTTPS configuration is invalid")
	}
	seen := make(map[string]bool, 5)
	for decoder.More() {
		keyToken, keyErr := decoder.Token()
		key, ok := keyToken.(string)
		if keyErr != nil || !ok || seen[key] {
			return Config{}, errors.New("trusted HTTPS configuration is invalid")
		}
		seen[key] = true
		switch key {
		case "provider":
			err = decoder.Decode(&config.Provider)
		case "domain":
			err = decoder.Decode(&config.Domain)
		case "token":
			err = decoder.Decode(&config.Token)
		case "address":
			err = decoder.Decode(&config.Address)
		case "termsAccepted":
			err = decoder.Decode(&config.Terms)
		default:
			return Config{}, errors.New("trusted HTTPS configuration is invalid")
		}
		if err != nil {
			return Config{}, errors.New("trusted HTTPS configuration is invalid")
		}
	}
	_, err = decoder.Token()
	if err == nil {
		err = decoder.Decode(&struct{}{})
	}
	if !errors.Is(err, io.EOF) {
		return Config{}, errors.New("trusted HTTPS configuration must contain one object")
	}
	if config != (Config{}) && config.Provider == "" {
		config.Provider = ProviderDuckDNS
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

// Encode validates and returns the canonical secret document.
func (config Config) Encode() (string, error) {
	if config != (Config{}) && config.Provider == "" {
		config.Provider = ProviderDuckDNS
	}
	if err := config.Validate(); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(config)
	return string(encoded), err
}

// ProviderName returns the configured provider. Legacy documents use DuckDNS.
func (config Config) ProviderName() string {
	if config == (Config{}) {
		return ""
	}
	if config.Provider == "" {
		return ProviderDuckDNS
	}
	return config.Provider
}

// Validate rejects ambiguous names, unsafe credentials, and routable targets.
func (config Config) Validate() error {
	if config == (Config{}) {
		return nil
	}
	provider := config.ProviderName()
	if provider != ProviderDeSEC && provider != ProviderDuckDNS {
		return errors.New("trusted HTTPS provider is not supported")
	}
	if err := validateDomain(provider, config.Domain); err != nil {
		return err
	}
	if err := validateToken(provider, config.Token); err != nil {
		return err
	}
	if err := validateAddress(config.Address); err != nil {
		return err
	}
	if !config.Terms {
		return errors.New("terms for Let's Encrypt were not accepted")
	}
	return nil
}

func validateAddress(address string) error {
	parsed, err := netip.ParseAddr(address)
	if err == nil {
		if !parsed.Is4() || !parsed.IsPrivate() {
			return errors.New("trusted HTTPS address must be a private IPv4 address or a local hostname")
		}
		return nil
	}
	if validHostname(address) {
		return nil
	}
	return errors.New("trusted HTTPS address must be a private IPv4 address or a local hostname")
}

func validateDomain(provider, domain string) error {
	if provider == ProviderDuckDNS && !validLabel(domain) {
		return errors.New("DuckDNS hostname must be one lowercase label or a duckdns.org name")
	}
	if provider == ProviderDeSEC && !validHostname(domain) {
		return errors.New("trusted HTTPS hostname must be a lowercase full domain name")
	}
	return nil
}

func validateToken(provider, token string) error {
	if provider == ProviderDuckDNS && (len(token) < 32 || len(token) > 128 || !safeDuckDNSToken(token)) {
		return errors.New("DuckDNS token must contain 32 to 128 safe characters")
	}
	if provider == ProviderDeSEC && (len(token) < 16 || len(token) > 512 || !safeHeaderValue(token)) {
		return errors.New("DNS provider token must contain 16 to 512 visible characters")
	}
	return nil
}

// Hostname is the public name covered by the trusted certificate.
func (config Config) Hostname() string {
	if config.Domain == "" {
		return ""
	}
	if config.ProviderName() == ProviderDuckDNS {
		return config.Domain + duckDNSSuffix
	}
	return config.Domain
}

func validHostname(hostname string) bool {
	if len(hostname) > 253 || !strings.Contains(hostname, ".") || hostname != strings.ToLower(strings.TrimSpace(hostname)) {
		return false
	}
	for _, label := range strings.Split(hostname, ".") {
		if !validLabel(label) {
			return false
		}
	}
	return true
}

func validLabel(label string) bool {
	if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	return strings.IndexFunc(label, func(character rune) bool { return !lowercaseLabelCharacter(character) }) == -1
}

func safeDuckDNSToken(token string) bool {
	return strings.IndexFunc(token, func(character rune) bool { return !duckDNSTokenCharacter(character) }) == -1
}

func lowercaseLabelCharacter(character rune) bool {
	return character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-'
}

func duckDNSTokenCharacter(character rune) bool {
	return lowercaseLabelCharacter(character) || character >= 'A' && character <= 'Z' || character == '_'
}

func safeHeaderValue(value string) bool {
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}
