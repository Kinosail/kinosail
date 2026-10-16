package auditjournal

import (
	"log/slog"
	"net/netip"
	"strconv"
	"time"
)

const (
	securityEventBurst     = 20
	securityEventWindow    = time.Minute
	trackedSecuritySources = 4096
	suppressedDetail       = "suppressed"
)

type floodSource struct {
	reset      time.Time
	recorded   int
	suppressed int
}

type floodGuard struct {
	sources map[string]floodSource
	swept   time.Time
}

func (guard *floodGuard) admit(event *Event, now time.Time) bool {
	if event.Action != "access.denied" && event.Action != "security.tripwire" {
		return true
	}
	key := guard.key(event.Action, event.Remote, now)
	state := guard.sources[key]
	if now.After(state.reset) || now.Before(state.reset.Add(-securityEventWindow)) {
		state = floodSource{reset: now.Add(securityEventWindow), suppressed: state.suppressed}
	}
	if state.recorded >= securityEventBurst {
		state.suppressed++
		guard.sources[key] = state
		if state.suppressed == 1 {
			slog.Debug("activity journal is aggregating repeated security events", "action", event.Action, "window_seconds", int(securityEventWindow.Seconds()))
		}
		return false
	}
	state.recorded++
	if _, exists := event.Details[suppressedDetail]; state.suppressed > 0 && !exists && len(event.Details) < 128 {
		if event.Details == nil {
			event.Details = make(map[string]string, 1)
		}
		event.Details[suppressedDetail] = strconv.Itoa(state.suppressed)
		state.suppressed = 0
	}
	guard.sources[key] = state
	return true
}

func (guard *floodGuard) key(action, remote string, now time.Time) string {
	if guard.sources == nil {
		guard.sources = make(map[string]floodSource)
	}
	key := action + " " + securitySource(remote)
	if _, found := guard.sources[key]; found || len(guard.sources) < trackedSecuritySources {
		return key
	}
	if now.Sub(guard.swept) >= securityEventWindow || now.Before(guard.swept) {
		guard.swept = now
		for candidate, state := range guard.sources {
			if now.After(state.reset) {
				delete(guard.sources, candidate)
			}
		}
	}
	if len(guard.sources) < trackedSecuritySources {
		return key
	}
	return action + " overflow"
}

func securitySource(remote string) string {
	address, err := netip.ParseAddr(remote)
	if err != nil {
		return "unparsed"
	}
	address = address.Unmap().WithZone("")
	if address.Is6() {
		return netip.PrefixFrom(address, 64).Masked().String()
	}
	return address.String()
}
