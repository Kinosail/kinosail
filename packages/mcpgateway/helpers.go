package mcpgateway

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"slices"
)

func writeJSON(writer http.ResponseWriter, value any, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func trustedURL(endpoint *url.URL) bool {
	return endpoint != nil && endpoint.Host != "" && endpoint.User == nil && endpoint.RawQuery == "" && endpoint.Fragment == "" && (endpoint.Scheme == "https" || endpoint.Scheme == "http" && loopbackHost(endpoint.Hostname()))
}

func loopbackHost(host string) bool {
	return host == "localhost" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

func contains(values []string, expected string) bool { return slices.Contains(values, expected) }
