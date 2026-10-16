package library

import (
	"slices"
	"strconv"
	"strings"
	"sync"
)

const cachedPolicyViews = 4

type Snapshot struct {
	items      []Item
	byID       map[string]int
	generation uint64
	all        *View
	mu         sync.Mutex
	views      map[string]*policyView
	used       uint64
}

type policyView struct {
	once sync.Once
	view *View
	used uint64
}

func NewSnapshot(items []Item, generation uint64) *Snapshot {
	items = slices.Clip(items)
	byID := make(map[string]int, len(items))
	for position, item := range items {
		if _, found := byID[item.ID]; !found {
			byID[item.ID] = position
		}
	}
	return &Snapshot{items: items, byID: byID, generation: generation, all: newView(items, byID), views: make(map[string]*policyView)}
}

func (snapshot *Snapshot) Generation() uint64 { return snapshot.generation }

func (snapshot *Snapshot) Len() int { return len(snapshot.items) }

func (snapshot *Snapshot) Items() []Item { return snapshot.items }

func (snapshot *Snapshot) DistinctIDs() bool { return len(snapshot.byID) == len(snapshot.items) }

func (snapshot *Snapshot) Find(id string) (Item, bool) {
	position, found := snapshot.byID[id]
	if !found {
		return Item{}, false
	}
	return snapshot.items[position], true
}

func (snapshot *Snapshot) View(policy Policy) *View {
	key, everything := policy.viewKey()
	if everything {
		return snapshot.all
	}
	snapshot.mu.Lock()
	entry := snapshot.views[key]
	if entry == nil {
		if len(snapshot.views) >= cachedPolicyViews {
			snapshot.evictView()
		}
		entry = &policyView{}
		snapshot.views[key] = entry
	}
	snapshot.used++
	entry.used = snapshot.used
	snapshot.mu.Unlock()
	entry.once.Do(func() { entry.view = snapshot.filter(policy) })
	return entry.view
}

func (snapshot *Snapshot) evictView() {
	oldest := ""
	for key, entry := range snapshot.views {
		if oldest == "" || entry.used < snapshot.views[oldest].used {
			oldest = key
		}
	}
	delete(snapshot.views, oldest)
}

func (snapshot *Snapshot) filter(policy Policy) *View {
	allowed := make([]int, 0, len(snapshot.items))
	for position := range snapshot.items {
		if policy.Allows(snapshot.items[position]) {
			allowed = append(allowed, position)
		}
	}
	if len(allowed) == len(snapshot.items) {
		return snapshot.all
	}
	items := make([]Item, len(allowed))
	byID := make(map[string]int, len(allowed))
	for position, source := range allowed {
		items[position] = snapshot.items[source]
		if _, found := byID[items[position].ID]; !found {
			byID[items[position].ID] = position
		}
	}
	return newView(items, byID)
}

func (policy Policy) viewKey() (string, bool) {
	if policy.Owner {
		return "", true
	}
	everyLibrary := slices.Contains(policy.Libraries, "all")
	var key strings.Builder
	key.WriteString(strconv.Quote(policy.Rating))
	if everyLibrary {
		key.WriteString(" all")
	} else {
		sorted := slices.Clone(policy.Libraries)
		slices.Sort(sorted)
		for _, name := range slices.Compact(sorted) {
			key.WriteString(" " + strconv.Quote(name))
		}
	}
	return key.String(), everyLibrary && policy.Rating == "all"
}
