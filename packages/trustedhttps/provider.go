package trustedhttps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Check verifies the provider token by updating the configured DNS address.
func Check(ctx context.Context, config Config, dependencies ...Dependencies) error {
	if len(dependencies) > 1 {
		return errors.New("trusted HTTPS check accepts at most one dependency set")
	}
	if err := config.Validate(); err != nil {
		return err
	}
	dependency := Dependencies{}
	if len(dependencies) != 0 {
		dependency = dependencies[0]
	}
	if dependency.Client == nil {
		dependency.Client = defaultHTTPClient()
	}
	provider, err := newDNSProvider(config, dependency.Client, providerEndpoints{duckDNS: dependency.UpdateURL, deSEC: dependency.DeSECURL})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return provider.UpdateAddress(ctx)
}

const (
	duckDNSUpdateURL = "https://www.duckdns.org/update"
	deSECAPIURL      = "https://desec.io"
)

type dnsProvider interface {
	UpdateAddress(context.Context) error
	PresentTXT(context.Context, string) error
	CleanupTXT(context.Context) error
}

type providerEndpoints struct {
	duckDNS  string
	deSEC    string
	lookupIP func(context.Context, string) ([]net.IP, error)
}

func (endpoints providerEndpoints) defaults() providerEndpoints {
	if endpoints.duckDNS == "" {
		endpoints.duckDNS = duckDNSUpdateURL
	}
	if endpoints.deSEC == "" {
		endpoints.deSEC = deSECAPIURL
	}
	if endpoints.lookupIP == nil {
		endpoints.lookupIP = func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip4", host)
		}
	}
	return endpoints
}

func newDNSProvider(config Config, client *http.Client, endpoints providerEndpoints) (dnsProvider, error) {
	endpoints = endpoints.defaults()
	switch config.ProviderName() {
	case ProviderDuckDNS:
		endpoint, err := providerEndpoint(endpoints.duckDNS)
		if err != nil {
			return nil, errors.New("DuckDNS update address must be an HTTPS URL")
		}
		return duckDNSProvider{config: config, client: client, endpoint: endpoint.String(), lookupIP: endpoints.lookupIP}, nil
	case ProviderDeSEC:
		endpoint, err := providerEndpoint(endpoints.deSEC)
		if err != nil {
			return nil, errors.New("deSEC API address must be an HTTPS URL")
		}
		return &deSECProvider{config: config, client: client, endpoint: endpoint, lookupIP: endpoints.lookupIP, marshal: func(payload deSECPayload) ([]byte, error) { return json.Marshal(payload) }}, nil
	default:
		return nil, errors.New("trusted HTTPS provider is not supported")
	}
}

func providerEndpoint(raw string) (*url.URL, error) {
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, errors.New("invalid provider endpoint")
	}
	return endpoint, nil
}

type duckDNSProvider struct {
	config   Config
	client   *http.Client
	endpoint string
	lookupIP func(context.Context, string) ([]net.IP, error)
}

func (provider duckDNSProvider) UpdateAddress(ctx context.Context) error {
	config, err := provider.config.withResolvedAddress(ctx, provider.lookupIP)
	if err != nil {
		return err
	}
	return updateDuckDNS(ctx, provider.client, provider.endpoint, config, "", false)
}

func (provider duckDNSProvider) PresentTXT(ctx context.Context, value string) error {
	return updateDuckDNS(ctx, provider.client, provider.endpoint, provider.config, value, false)
}

func (provider duckDNSProvider) CleanupTXT(ctx context.Context) error {
	return updateDuckDNS(ctx, provider.client, provider.endpoint, provider.config, "", true)
}

type deSECProvider struct {
	config   Config
	client   *http.Client
	endpoint *url.URL
	lookupIP func(context.Context, string) ([]net.IP, error)
	marshal  func(deSECPayload) ([]byte, error)
}

type deSECPayload struct {
	Subname string   `json:"subname"`
	Type    string   `json:"type"`
	TTL     int      `json:"ttl"`
	Records []string `json:"records"`
}

func (provider *deSECProvider) UpdateAddress(ctx context.Context) error {
	config, err := provider.config.withResolvedAddress(ctx, provider.lookupIP)
	if err != nil {
		return err
	}
	return provider.put(ctx, "@", "A", []string{config.Address})
}

func (provider *deSECProvider) PresentTXT(ctx context.Context, value string) error {
	return provider.put(ctx, "_acme-challenge", "TXT", []string{strconv.Quote(value)})
}

func (provider *deSECProvider) CleanupTXT(ctx context.Context) error {
	request := provider.request(ctx, http.MethodDelete, "_acme-challenge", "TXT", nil)
	return providerResponse(provider.client, request, "deSEC", http.StatusNoContent, http.StatusNotFound)
}

func (provider *deSECProvider) put(ctx context.Context, subname, recordType string, records []string) error {
	canonicalSubname := subname
	if subname == "@" {
		canonicalSubname = ""
	}
	body, err := provider.marshal(deSECPayload{Subname: canonicalSubname, Type: recordType, TTL: 3600, Records: records})
	if err != nil {
		return errors.New("create deSEC request")
	}
	request := provider.request(ctx, http.MethodPut, subname, recordType, body)
	return providerResponse(provider.client, request, "deSEC", http.StatusOK, http.StatusCreated)
}

func (provider *deSECProvider) request(ctx context.Context, method, subname, recordType string, body []byte) *http.Request {
	path := "/api/v1/domains/" + url.PathEscape(provider.config.Hostname()) + "/rrsets/" + url.PathEscape(subname) + "/" + recordType + "/"
	var requestBody io.ReadCloser
	if body != nil {
		requestBody = io.NopCloser(bytes.NewReader(body))
	}
	request := (&http.Request{Method: method, URL: provider.endpoint.ResolveReference(&url.URL{Path: path}), Header: make(http.Header), Body: requestBody, ContentLength: int64(len(body))}).WithContext(ctx)
	request.Header.Set("Authorization", "Token "+provider.config.Token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}

func providerResponse(client *http.Client, request *http.Request, name string, statuses ...int) error {
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("%s request failed", name)
	}
	defer response.Body.Close()
	_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if readErr != nil || !containsStatus(statuses, response.StatusCode) {
		return fmt.Errorf("%s rejected the request", name)
	}
	return nil
}

func containsStatus(statuses []int, status int) bool {
	for _, allowed := range statuses {
		if status == allowed {
			return true
		}
	}
	return false
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
