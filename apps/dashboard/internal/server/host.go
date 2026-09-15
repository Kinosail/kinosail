package server

import (
	"net"
	"net/http"
	"strings"
)

func protectHost(trustedHosts []string, next http.Handler) http.Handler {
	trusted := make(map[string]bool, len(trustedHosts))
	for _, host := range trustedHosts {
		trusted[strings.ToLower(strings.TrimSuffix(host, "."))] = true
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		host := request.Host
		if parsed, _, err := net.SplitHostPort(host); err == nil {
			host = parsed
		}
		host = strings.ToLower(strings.TrimSuffix(strings.Trim(host, "[]"), "."))
		address := net.ParseIP(host)
		allowedAddress := address != nil && (address.IsLoopback() || address.IsPrivate())
		if host == "" || host != "localhost" && !allowedAddress && !trusted[host] {
			http.Error(writer, "request host is not allowed", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(writer, request)
	})
}
