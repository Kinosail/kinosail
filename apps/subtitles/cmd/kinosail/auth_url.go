package main

import (
	"net"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
)

func configuredAuthURL(configured configuration.Snapshot) string {
	if raw := configured.String("auth.url"); raw != "" {
		return raw
	}
	if origin, err := configuration.TrustedOrigin(configured.String("tls.duckdns"), configured.String("listen")); err == nil && origin != "" {
		return origin
	}
	scheme := "https"
	if !configured.Bool("tls.enabled") {
		scheme = "http"
	}
	_, port, err := net.SplitHostPort(configured.String("listen"))
	if err != nil {
		return scheme + "://localhost:38128"
	}
	return scheme + "://localhost:" + port
}
