package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func validPrivatePath(path string) bool {
	switch path {
	case "/setup", "/login", "/account", "/account/mfa/enable", "/?view=library",
		"/api/v1/subtitle-library?view=library", "/api/v1/subtitle-library?view=history",
		"/api/v1/subtitle-operations":
		return true
	}
	prefix := "/api/v1/subtitle-library/"
	if strings.HasPrefix(path, prefix) {
		remainder := strings.TrimPrefix(path, prefix)
		id, suffix, found := strings.Cut(remainder, "/")
		if !found || !itemID.MatchString(id) {
			return false
		}
		switch suffix {
		case "inspect?language=en", "export?language=en&format=srt", "preview", "apply":
			return true
		}
	}
	id := strings.TrimPrefix(path, "/api/v1/subtitle-operations/")
	return path != id && operationID.MatchString(id)
}

func (f *fixture) privateRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if !validPrivatePath(path) {
		return nil, errors.New("private route is not admitted")
	}
	route, query, _ := strings.Cut(path, "?")
	endpoint := url.URL{Scheme: "https", Host: f.authority, Path: route, RawQuery: query}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Origin", f.origin)
	request.Header.Set("User-Agent", "R06-private-control")
	return request, nil
}

func (f *fixture) privateClient(timeout time.Duration) *http.Client {
	client := *f.tls.Client()
	original, ok := client.Transport.(*http.Transport)
	if !ok {
		client.Transport = deniedTransport{}
		return &client
	}
	transport := original.Clone()
	transport.DialContext = f.dialOwned
	client.Transport = transport
	client.Timeout = timeout
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client
}

func (f *fixture) dialOwned(ctx context.Context, network, address string) (net.Conn, error) {
	if address != f.authority {
		return nil, errors.New("private transport authority rejected")
	}
	dialer := net.Dialer{Timeout: 2 * time.Second}
	return dialer.DialContext(ctx, network, address)
}

func (f *fixture) admittedPrivateRequest(request *http.Request) bool {
 if request.URL == nil { return false }
 endpoint := request.URL
 if endpoint.Scheme != "https" || endpoint.Host != f.authority || endpoint.User != nil { return false }
 address := net.ParseIP(endpoint.Hostname())
 if address == nil || !address.IsLoopback() { return false }
 return validPrivatePath(endpoint.RequestURI()) && endpoint.Fragment == "" && endpoint.Opaque == "" && endpoint.RawPath == ""
}

func (f *fixture) closeFailedPrivateResponse(response *http.Response) {
 if response == nil { return }
 if err := response.Body.Close(); err != nil { f.failBoundary() }
}

func readPrivateResponse(response *http.Response) ([]byte, error) {
	data, readErr := io.ReadAll(io.LimitReader(response.Body, privateResponseLimit+1))
	return data, errors.Join(readErr, response.Body.Close())
}

type deniedTransport struct{}

func (deniedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("private TLS transport unavailable")
}
