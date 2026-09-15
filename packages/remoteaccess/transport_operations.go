package remoteaccess

import (
	"context"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
)

type transportOperations struct {
	after           func(time.Duration) <-chan time.Time
	configureServer func(*http.Server, *http2.Server) error
	listen          func(context.Context, string, string) (net.Listener, error)
	refreshTicks    func() (<-chan time.Time, func())
}

func defaultTransportOperations() transportOperations {
	return transportOperations{
		after:           time.After,
		configureServer: http2.ConfigureServer,
		listen:          (&net.ListenConfig{}).Listen,
		refreshTicks: func() (<-chan time.Time, func()) {
			ticker := time.NewTicker(5 * time.Minute)
			return ticker.C, ticker.Stop
		},
	}
}
