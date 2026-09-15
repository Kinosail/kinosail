package dashboard

import (
	"context"
	"errors"
)

type probeFlightKey struct {
	id       string
	revision uint64
}

// ProbeOne shares one bounded network check without giving any caller ownership of its lifetime.
func (prober *Prober) ProbeOne(ctx context.Context, id string) (Health, error) {
	if err := ctx.Err(); err != nil {
		return Health{}, err
	}
	key, found := prober.service.probeIntent(id)
	if !found {
		return Health{}, ErrNotFound
	}
	prober.flightMu.Lock()
	flight := prober.flights[key]
	if flight == nil {
		flightCtx, cancel := context.WithTimeout(context.Background(), probeTimeout)
		flight = &probeFlight{done: make(chan struct{}), cancel: cancel}
		prober.flights[key] = flight
		go prober.runFlight(flightCtx, key, flight) //nolint:contextcheck // A shared flight has its own timeout; one caller must not cancel other waiters.
	}
	flight.waiters++
	prober.flightMu.Unlock()

	select {
	case <-flight.done:
		return flight.health, flight.err
	case <-ctx.Done():
		prober.leaveFlight(key, flight)
		return Health{}, ctx.Err()
	}
}

func (prober *Prober) leaveFlight(key probeFlightKey, flight *probeFlight) {
	prober.flightMu.Lock()
	defer prober.flightMu.Unlock()
	if prober.flights[key] != flight {
		return
	}
	flight.waiters--
}

func (prober *Prober) runFlight(ctx context.Context, key probeFlightKey, flight *probeFlight) {
	select {
	case prober.slots <- struct{}{}:
		defer func() { <-prober.slots }()
	case <-ctx.Done():
		flight.err = ctx.Err()
		prober.finishFlight(key, flight)
		return
	}
	if !prober.startFlight(key, flight) {
		return
	}
	app, generation, err := prober.service.beginProbe(key)
	if err != nil {
		flight.err = err
		prober.finishFlight(key, flight)
		return
	}
	health := prober.check(ctx, app.HealthURL)
	if !errors.Is(ctx.Err(), context.Canceled) {
		prober.service.setHealth(key.id, generation, health)
		flight.health = health
	} else {
		flight.err = context.Canceled
	}
	prober.finishFlight(key, flight)
}

func (prober *Prober) startFlight(key probeFlightKey, flight *probeFlight) bool {
	prober.flightMu.Lock()
	defer prober.flightMu.Unlock()
	if prober.flights[key] != flight {
		return false
	}
	if flight.waiters > 0 {
		return true
	}
	delete(prober.flights, key)
	flight.err = context.Canceled
	flight.cancel()
	close(flight.done)
	return false
}

func (service *Service) probeIntent(id string) (probeFlightKey, bool) {
	service.mu.RLock()
	defer service.mu.RUnlock()
	for _, app := range service.board.Apps {
		if app.ID == id && app.CheckEnabled {
			return probeFlightKey{id: id, revision: service.probeRevision[id]}, true
		}
	}
	return probeFlightKey{}, false
}

func (service *Service) beginProbe(key probeFlightKey) (App, uint64, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.probeRevision[key.id] != key.revision {
		return App{}, 0, ErrConflict
	}
	for _, app := range service.board.Apps {
		if app.ID == key.id && app.CheckEnabled {
			service.healthGeneration[key.id]++
			generation := service.healthGeneration[key.id]
			service.health[key.id] = Health{State: "checking", Explanation: "Check in progress"}
			return app, generation, nil
		}
	}
	return App{}, 0, ErrNotFound
}

func (prober *Prober) finishFlight(key probeFlightKey, flight *probeFlight) {
	prober.flightMu.Lock()
	if prober.flights[key] == flight {
		delete(prober.flights, key)
	}
	flight.cancel()
	close(flight.done)
	prober.flightMu.Unlock()
}
