package auditjournal

import (
	"crypto/rand"
	"log/slog"
	"net/netip"
	"strconv"
	"time"
)

const (
	securityEventBurst      = 20
	securityEventWindow     = time.Minute
	trackedSecuritySources  = 4096
	trackedSecurityNetworks = 1024
	suppressedDetail        = "suppressed"
)

type floodSource struct {
	reset      time.Time
	recorded   int
	suppressed int
	last       Event
}

type floodGuard struct {
	sources    map[string]floodSource
	networks   map[string]floodSource
	swept      time.Time
	overflowed time.Time
}

func (guard *floodGuard) admit(event *Event, now time.Time) (bool, []Event) {
	summaries := guard.sweep(now)
	if event.Action != "access.denied" && event.Action != "security.tripwire" {
		return true, summaries
	}
	table, key := guard.bucket(event.Action, event.Remote, now)
	state := table[key]
	if now.After(state.reset) || now.Before(state.reset.Add(-securityEventWindow)) {
		state = floodSource{reset: now.Add(securityEventWindow), suppressed: state.suppressed}
	}
	state.last = floodSummaryTemplate(*event)
	if state.recorded >= securityEventBurst {
		state.suppressed++
		table[key] = state
		if state.suppressed == 1 {
			slog.Debug("activity journal is aggregating repeated security events", "action", event.Action, "window_seconds", int(securityEventWindow.Seconds()))
		}
		return false, summaries
	}
	state.recorded++
	if _, exists := event.Details[suppressedDetail]; state.suppressed > 0 && !exists && len(event.Details) < 128 {
		if event.Details == nil {
			event.Details = make(map[string]string, 1)
		}
		event.Details[suppressedDetail] = strconv.Itoa(state.suppressed)
		state.suppressed = 0
	}
	table[key] = state
	return true, summaries
}

func (guard *floodGuard) bucket(action, remote string, now time.Time) (map[string]floodSource, string) {
	if guard.sources == nil {
		guard.sources, guard.networks = make(map[string]floodSource), make(map[string]floodSource)
	}
	key := action + " " + securitySource(remote, 64, 32)
	if _, found := guard.sources[key]; found || len(guard.sources) < trackedSecuritySources {
		return guard.sources, key
	}
	key = action + " " + securitySource(remote, 48, 24)
	if _, found := guard.networks[key]; !found && len(guard.networks) >= trackedSecurityNetworks {
		key = action + " overflow"
	}
	if !now.Before(guard.overflowed) {
		guard.overflowed = now.Add(securityEventWindow)
		slog.Warn("activity journal is aggregating security events by network", "action", action, "tracked_sources", len(guard.sources), "tracked_networks", len(guard.networks), "window_seconds", int(securityEventWindow.Seconds()))
	}
	return guard.networks, key
}

func (guard *floodGuard) sweep(now time.Time) []Event {
	if now.Sub(guard.swept) < securityEventWindow && !now.Before(guard.swept.Add(-securityEventWindow)) {
		return nil
	}
	guard.swept = now
	var summaries []Event
	for _, table := range []map[string]floodSource{guard.sources, guard.networks} {
		for key, state := range table {
			if !now.After(state.reset) {
				continue
			}
			if state.suppressed > 0 {
				summaries = append(summaries, floodSummary(state, now))
			}
			delete(table, key)
		}
	}
	return summaries
}

func floodSummaryTemplate(event Event) Event {
	summary := Event{Category: event.Category, Action: event.Action, Result: event.Result, Remote: event.Remote}
	if quarantined, found := event.Details["quarantined"]; found {
		summary.Details = map[string]string{"quarantined": quarantined}
	}
	return summary
}

func floodSummary(state floodSource, now time.Time) Event {
	summary := state.last
	summary.ID, summary.Time = rand.Text(), now.UTC().Format(time.RFC3339Nano)
	details := make(map[string]string, len(summary.Details)+1)
	for key, value := range summary.Details {
		details[key] = value
	}
	details[suppressedDetail] = strconv.Itoa(state.suppressed)
	summary.Details = details
	return summary
}

func securitySource(remote string, ipv6Bits, ipv4Bits int) string {
	address, err := netip.ParseAddr(remote)
	if err != nil {
		return "unparsed"
	}
	address = address.Unmap().WithZone("")
	if address.Is6() {
		return netip.PrefixFrom(address, ipv6Bits).Masked().String()
	}
	return netip.PrefixFrom(address, ipv4Bits).Masked().String()
}

type admittedEvent struct {
	event   Event
	created time.Time
}

func summarizedEvents(summaries []Event, created time.Time) []admittedEvent {
	events := make([]admittedEvent, 0, len(summaries)+1)
	for _, summary := range summaries {
		if validEvent(summary, false) {
			events = append(events, admittedEvent{summary, created})
		}
	}
	return events
}

func (journal *Journal) summarizeSecurityEvents(done <-chan struct{}) {
	ticker := time.NewTicker(securityEventWindow)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case now := <-ticker.C:
			journal.store(journal.expiredSecuritySummaries(now.UTC()))
		}
	}
}

func (journal *Journal) expiredSecuritySummaries(now time.Time) []admittedEvent {
	journal.mu.Lock()
	defer journal.mu.Unlock()
	if !journal.writable {
		return nil
	}
	return summarizedEvents(journal.flood.sweep(now), now)
}
