package trustedhttps

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/crypto/acme"
)

const accountKeyName = "acme-account.pem"

type acmeClient interface {
	Register(context.Context, *acme.Account, func(string) bool) (*acme.Account, error)
	AuthorizeOrder(context.Context, []acme.AuthzID, ...acme.OrderOption) (*acme.Order, error)
	GetAuthorization(context.Context, string) (*acme.Authorization, error)
	DNS01ChallengeRecord(string) (string, error)
	Accept(context.Context, *acme.Challenge) (*acme.Challenge, error)
	WaitAuthorization(context.Context, string) (*acme.Authorization, error)
	WaitOrder(context.Context, string) (*acme.Order, error)
	CreateOrderCert(context.Context, string, []byte, bool) ([][]byte, string, error)
}

func (config Config) withResolvedAddress(ctx context.Context, lookupIP func(context.Context, string) ([]net.IP, error)) (Config, error) { //nolint:cyclop // Address resolution rejects every ambiguous or non-private result before mutation.
	if _, err := netip.ParseAddr(config.Address); err == nil {
		if err := validateAddress(config.Address); err != nil {
			return Config{}, err
		}
		return config, nil
	}
	if lookupIP == nil {
		lookupIP = func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip4", host)
		}
	}
	addresses, err := lookupIP(ctx, config.Address)
	if err != nil {
		return Config{}, errors.New("trusted HTTPS address could not be resolved")
	}
	var resolved netip.Addr
	for _, address := range addresses {
		parsed, ok := netip.AddrFromSlice(address.To4())
		if !ok || !parsed.Is4() {
			continue
		}
		if !parsed.IsPrivate() {
			return Config{}, errors.New("trusted HTTPS hostname resolved to a non-private IPv4 address")
		}
		if resolved.IsValid() && resolved != parsed {
			return Config{}, errors.New("trusted HTTPS hostname resolved to multiple private IPv4 addresses")
		}
		resolved = parsed
	}
	if !resolved.IsValid() {
		return Config{}, errors.New("trusted HTTPS hostname did not resolve to a private IPv4 address")
	}
	config.Address = resolved.String()
	return config, nil
}

func obtainCertificate(ctx context.Context, config Config, dataDir string, httpClient *http.Client, directoryURL string, provider dnsProvider) ([]byte, error) {
	accountKey, err := accountKey(filepath.Join(dataDir, accountKeyName))
	if err != nil {
		return nil, err
	}
	if directoryURL == "" {
		directoryURL = acme.LetsEncryptURL
	}
	client := &acme.Client{Key: accountKey, DirectoryURL: directoryURL, HTTPClient: httpClient, UserAgent: "Kinosail trusted HTTPS"}
	return issue(ctx, config, client, provider, waitForTXT)
}

func issue(ctx context.Context, config Config, client acmeClient, provider dnsProvider, waitTXT func(context.Context, string, string) error) ([]byte, error) {
	if _, err := client.Register(ctx, &acme.Account{}, acme.AcceptTOS); err != nil && !errors.Is(err, acme.ErrAccountAlreadyExists) {
		return nil, errors.New("register certificate account")
	}
	hostname := config.Hostname()
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(hostname))
	if err != nil || order == nil || len(order.AuthzURLs) != 1 || order.FinalizeURL == "" || order.URI == "" {
		return nil, errors.New("create certificate order")
	}
	for _, authorizationURL := range order.AuthzURLs {
		if err := authorizeDNS(ctx, config, client, provider, waitTXT, authorizationURL); err != nil {
			return nil, err
		}
	}
	return finalizeCertificate(ctx, client, order, hostname)
}

func authorizeDNS(ctx context.Context, config Config, client acmeClient, provider dnsProvider, waitTXT func(context.Context, string, string) error, authorizationURL string) error { //nolint:cyclop // ACME challenge setup and cleanup form one fail-closed transaction.
	authorization, err := client.GetAuthorization(ctx, authorizationURL)
	if err != nil || authorization == nil || len(authorization.Challenges) > 32 {
		return errors.New("read certificate authorization")
	}
	if authorization.Status == acme.StatusValid {
		return nil
	}
	challenge := dnsChallenge(authorization.Challenges)
	if challenge == nil {
		return errors.New("certificate authority did not offer DNS validation")
	}
	value, err := client.DNS01ChallengeRecord(challenge.Token)
	if err != nil || value == "" || len(value) > 512 {
		return errors.New("create certificate DNS validation")
	}
	hostname := config.Hostname()
	slog.Info("trusted HTTPS DNS challenge update started", "provider", config.ProviderName(), "hostname", hostname)
	if err := provider.PresentTXT(ctx, value); err != nil {
		return err
	}
	slog.Info("trusted HTTPS DNS challenge published", "provider", config.ProviderName(), "hostname", hostname)
	err = completeDNSAuthorization(ctx, client, waitTXT, authorizationURL, hostname, value, challenge)
	slog.Info("trusted HTTPS DNS challenge cleanup started", "provider", config.ProviderName(), "hostname", hostname)
	cleanupErr := provider.CleanupTXT(context.WithoutCancel(ctx))
	if err != nil {
		return errors.New("complete certificate DNS validation")
	}
	if cleanupErr != nil {
		return cleanupErr
	}
	slog.Info("trusted HTTPS DNS challenge complete", "provider", config.ProviderName(), "hostname", hostname)
	return nil
}

func dnsChallenge(challenges []*acme.Challenge) *acme.Challenge {
	for _, challenge := range challenges {
		if challenge != nil && challenge.Type == "dns-01" {
			return challenge
		}
	}
	return nil
}

func completeDNSAuthorization(ctx context.Context, client acmeClient, waitTXT func(context.Context, string, string) error, authorizationURL, hostname, value string, challenge *acme.Challenge) error {
	slog.Info("trusted HTTPS DNS challenge waiting", "hostname", hostname)
	if err := waitTXT(ctx, "_acme-challenge."+hostname, value); err != nil {
		return err
	}
	if _, err := client.Accept(ctx, challenge); err != nil {
		return err
	}
	completed, err := client.WaitAuthorization(ctx, authorizationURL)
	if err != nil {
		return err
	}
	if completed == nil || completed.Status != acme.StatusValid {
		return errors.New("authorization did not become valid")
	}
	return nil
}

func finalizeCertificate(ctx context.Context, client acmeClient, order *acme.Order, hostname string) ([]byte, error) {
	return finalizeCertificateWith(ctx, client, order, hostname, defaultIdentityOperations(), time.Now())
}

func updateDuckDNS(ctx context.Context, client *http.Client, updateURL string, config Config, txt string, clear bool) error { //nolint:cyclop // Query construction and bounded response validation share one adapter boundary.
	endpoint, err := url.Parse(updateURL)
	if err != nil {
		return errors.New("create DuckDNS update")
	}
	query := endpoint.Query()
	query.Set("domains", config.Domain)
	query.Set("token", config.Token)
	if txt != "" || clear {
		query.Set("txt", txt)
		if clear {
			query.Set("clear", "true")
		}
	} else {
		query.Set("ip", config.Address)
	}
	endpoint.RawQuery = query.Encode()
	request := (&http.Request{Method: http.MethodGet, URL: endpoint, Header: make(http.Header)}).WithContext(ctx)
	response, err := client.Do(request)
	if err != nil {
		return errors.New("DuckDNS update failed")
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 65))
	if readErr != nil || response.StatusCode != http.StatusOK || len(body) > 64 || strings.TrimSpace(string(body)) != "OK" {
		return errors.New("DuckDNS rejected the update")
	}
	return nil
}

func waitForTXT(ctx context.Context, name, expected string) error {
	return waitForTXTWith(ctx, name, expected, net.DefaultResolver.LookupTXT, waitContext)
}

func waitForTXTWith(ctx context.Context, name, expected string, lookup func(context.Context, string) ([]string, error), wait func(context.Context, time.Duration) bool) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	for {
		values, err := lookup(ctx, name)
		if err == nil {
			if slices.Contains(values, expected) {
				return nil
			}
		}
		if !wait(ctx, 2*time.Second) {
			return errors.New("trusted HTTPS TXT record did not propagate")
		}
	}
}
