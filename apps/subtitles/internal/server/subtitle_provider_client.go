package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const providerJSONLimit = 2 << 20

var errProviderSessionExpired = errors.New("provider session expired")

type providerClient struct {
	name, base, domain string
	http               *http.Client
	health             *subtitleProviderHealthRegistry
}

type providerCall struct {
	route, method, endpoint string
	body                    []byte
	header                  http.Header
	expired                 func() bool
}

func newProviderClient(name, base, domain string, health *subtitleProviderHealthRegistry) *providerClient {
	client := &providerClient{name: name, base: base, domain: domain, health: health}
	client.http = localIntegrationHTTPClient(15 * time.Second)
	client.http.CheckRedirect = subtitleProviderRedirect(client.allowed)
	return client
}

func (client *providerClient) allowed(link string) bool {
	return trustedProviderLink(client.base, link, client.domain)
}

func (client *providerClient) endpoint(base, path string) (*url.URL, error) {
	endpoint, err := url.Parse(strings.TrimRight(base, "/") + "/" + path)
	if err != nil || !client.allowed(endpoint.String()) {
		return nil, fmt.Errorf("%s endpoint is invalid", client.name)
	}
	return endpoint, nil
}

func (client *providerClient) json(ctx context.Context, call providerCall, target any) error {
	if call.header == nil {
		call.header = http.Header{}
	}
	call.header.Set("Accept", "application/json")
	response, started, err := client.send(ctx, call)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized && call.expired != nil && call.expired() {
		client.health.observe(client.name, nil, errProviderSessionExpired)
		client.log(ctx, call, response.StatusCode, started, "expired")
		return fmt.Errorf("%s: %w", client.name, errProviderSessionExpired)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		client.health.observe(client.name, response, errors.New("request failed"))
		client.log(ctx, call, response.StatusCode, started, "rejected")
		return fmt.Errorf("%s rejected request", client.name)
	}
	err = decodeExternalJSON(response.Body, providerJSONLimit, target)
	client.health.observe(client.name, response, err)
	client.log(ctx, call, response.StatusCode, started, outcomeOf(err, "invalid"))
	return err
}

func (client *providerClient) file(ctx context.Context, call providerCall) ([]byte, error) {
	response, started, err := client.send(ctx, call)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := readSubtitle(response)
	availability := err
	if response.StatusCode == http.StatusOK && errors.Is(err, errSubtitleDownloadTooLarge) {
		availability = nil
	}
	client.health.observeFile(client.name, response, availability)
	outcome := outcomeOf(err, "rejected")
	if errors.Is(err, errSubtitleDownloadTooLarge) {
		outcome = "too_large"
	}
	client.log(ctx, call, response.StatusCode, started, outcome)
	return data, err
}

func (client *providerClient) send(ctx context.Context, call providerCall) (*http.Response, time.Time, error) {
	started := time.Now()
	if !client.allowed(call.endpoint) {
		client.log(ctx, call, 0, started, "untrusted")
		return nil, started, fmt.Errorf("%s link is not trusted", client.name)
	}
	if err := client.health.before(client.name); err != nil {
		client.log(ctx, call, 0, started, "deferred")
		return nil, started, err
	}
	var body io.Reader
	if len(call.body) > 0 {
		body = bytes.NewReader(call.body)
	}
	request, err := http.NewRequestWithContext(ctx, call.method, call.endpoint, body)
	if err != nil {
		return nil, started, err
	}
	for key, values := range call.header {
		request.Header[key] = values
	}
	request.Header.Set("User-Agent", "Kinosail Subtitles v0.1")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.http.Do(request)
	if err != nil {
		client.health.observe(client.name, nil, err)
		client.log(ctx, call, 0, started, "network")
		return nil, started, err
	}
	return response, started, nil
}

func (client *providerClient) log(ctx context.Context, call providerCall, status int, started time.Time, outcome string) {
	level := slog.LevelWarn
	if outcome == "ok" || outcome == "deferred" {
		level = slog.LevelDebug
	}
	slog.Log(context.WithoutCancel(ctx), level, "subtitle provider request", "operation", "subtitle.provider", "provider", client.name, "route", call.route, "outcome", outcome, "status", status, "duration_ms", time.Since(started).Milliseconds())
}

func outcomeOf(err error, failure string) string {
	if err != nil {
		return failure
	}
	return "ok"
}
