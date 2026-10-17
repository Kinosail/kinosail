package mediaprobe

import (
	"slices"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	probeMemoryLimit    = 8192
	firstProbeBackoff   = time.Minute
	maximumProbeBackoff = time.Hour
)

type accessEntry struct {
	version, intervals string
	points             []float64
}

type probeFailure struct {
	version string
	until   time.Time
	delay   time.Duration
}

func (probe *Probe) remember(id, version string, result Result) {
	if _, found := probe.cache[id]; !found && len(probe.cache) >= probeMemoryLimit {
		for evicted := range probe.cache {
			delete(probe.cache, evicted)
			delete(probe.versions, evicted)
			break
		}
	}
	result.RandomAccess = nil
	probe.cache[id], probe.versions[id] = result, version
}

func (probe *Probe) rememberAccess(id string, entry accessEntry) {
	if _, found := probe.access[id]; !found && len(probe.access) >= probeMemoryLimit {
		for evicted := range probe.access {
			delete(probe.access, evicted)
			break
		}
	}
	probe.access[id] = entry
}

func (probe *Probe) cachedAccess(id, version, intervals string) ([]float64, bool) {
	probe.mu.RLock()
	defer probe.mu.RUnlock()
	entry, found := probe.access[id]
	if !found || entry.version != version || entry.intervals != intervals {
		return nil, false
	}
	return slices.Clone(entry.points), true
}

func (probe *Probe) storeAccess(item library.Item, version, intervals string, points []float64) {
	probe.mu.Lock()
	probe.rememberAccess(item.ID, accessEntry{version: version, intervals: intervals, points: slices.Clone(points)})
	facts, found := probe.cache[item.ID]
	current := probe.versions[item.ID] == version
	probe.mu.Unlock()
	if found && current && version != "" {
		facts.RandomAccess = points
		probe.saveEntry(item, probeCacheEntry{Schema: 4, Version: version, Result: facts, RandomAccessFor: intervals})
	}
}

func (probe *Probe) failedRecently(id, version string) bool {
	failure, found := probe.failures[id]
	return found && failure.version == version && time.Now().Before(failure.until)
}

func (probe *Probe) recordFailure(id, version string) {
	delay := firstProbeBackoff
	if previous, found := probe.failures[id]; found && previous.version == version {
		delay = min(maximumProbeBackoff, previous.delay*2)
	}
	if _, found := probe.failures[id]; !found && len(probe.failures) >= probeMemoryLimit {
		for evicted := range probe.failures {
			delete(probe.failures, evicted)
			break
		}
	}
	probe.failures[id] = probeFailure{version: version, until: time.Now().Add(delay), delay: delay}
}
