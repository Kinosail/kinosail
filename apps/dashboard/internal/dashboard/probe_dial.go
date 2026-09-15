package dashboard

import (
	"context"
	"errors"
	"net"
	"time"
)

// Every address has already passed destination policy. Dial only pinned numeric
// addresses, with at most two attempts and a staggered alternate address family.
func dialProbeAddresses(ctx context.Context, network, port string, addresses []net.IPAddr, dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	if len(addresses) == 0 || len(addresses) > 64 {
		return nil, errors.New("probe address count is invalid")
	}
	ordered := append([]net.IPAddr(nil), addresses...)
	for index := 1; index < len(ordered); index++ {
		if (ordered[index].IP.To4() == nil) != (ordered[0].IP.To4() == nil) {
			ordered[1], ordered[index] = ordered[index], ordered[1]
			break
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		connection net.Conn
		err        error
	}
	results := make(chan result)
	launch := func(index int) {
		go func() {
			connection, err := dial(ctx, network, net.JoinHostPort(ordered[index].IP.String(), port))
			select {
			case results <- result{connection, err}:
			case <-ctx.Done():
				if connection != nil {
					_ = connection.Close()
				}
			}
		}()
	}
	launch(0)
	next, active := 1, 1
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	var last error
	for active > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case response := <-results:
			active--
			if response.err == nil && response.connection != nil {
				return response.connection, nil
			}
			if response.connection != nil {
				_ = response.connection.Close()
			}
			last = response.err
			if next < len(ordered) {
				launch(next)
				next++
				active++
			}
		case <-timer.C:
			if active < 2 && next < len(ordered) {
				launch(next)
				next++
				active++
			}
		}
	}
	if last == nil {
		last = errors.New("probe could not connect")
	}
	return nil, last
}
