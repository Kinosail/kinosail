package remoteaccess

import (
	"context"
	"net"
	"time"
)

type transportOperations struct {
	after        func(time.Duration) <-chan time.Time
	listen       func(context.Context, string, string) (net.Listener, error)
	refreshTicks func() (<-chan time.Time, func())
}

func defaultTransportOperations() transportOperations {
	return transportOperations{
		after:  time.After,
		listen: (&net.ListenConfig{}).Listen,
		refreshTicks: func() (<-chan time.Time, func()) {
			ticker := time.NewTicker(5 * time.Minute)
			return ticker.C, ticker.Stop
		},
	}
}
