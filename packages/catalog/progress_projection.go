package catalog

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type ResumeItem struct {
	library.Item
	Resume string
}
type PlaybackView struct {
	Profile  string `json:"profile"`
	Title    string `json:"title"`
	Position string `json:"position"`
	Updated  string `json:"updated"`
	when     time.Time
}

func ResumeAt(seconds float64) string {
	if seconds >= 0 && seconds < 60 {
		return fmt.Sprintf("Resume at 0:%02d", int(seconds))
	}
	minutes := int(seconds) / 60
	if minutes < 60 {
		return fmt.Sprintf("Resume at %dm", minutes)
	}
	return fmt.Sprintf("Resume at %dh %dm", minutes/60, minutes%60)
}

func ViewerProgress(values map[string]PlaybackState, profile string, owner bool, id string) PlaybackState {
	var buffer [128]byte
	key := append(append(append(buffer[:0], profile...), ':'), id...)
	value, found := values[string(key)]
	if !found && owner {
		value = values[id]
	}
	return value
}

func ActiveProgress(values map[string]PlaybackState, profile string, owner bool, items []library.Item) []ResumeItem {
	active := make([]ResumeItem, 0)
	for _, item := range items {
		value := ViewerProgress(values, profile, owner, item.ID)
		if value.Seconds > 0 && !value.Watched && !value.Dismissed {
			active = append(active, ResumeItem{item, ResumeAt(value.Seconds)})
		}
	}
	return active
}

func ProgressHistory(values map[string]PlaybackState, profile string, owner bool, items []library.Item) []library.Item {
	history, _ := ProgressHistoryPage(values, profile, owner, items, Page{})
	return history
}

func ProgressHistoryPage(values map[string]PlaybackState, profile string, owner bool, items []library.Item, page Page) ([]library.Item, int) {
	type played struct {
		position int
		updated  time.Time
	}
	history := make([]played, 0, min(len(items), len(values)))
	for position := range items {
		if updated := ViewerProgress(values, profile, owner, items[position].ID).Updated; !updated.IsZero() {
			history = append(history, played{position, updated})
		}
	}
	sort.SliceStable(history, func(left, right int) bool { return history[left].updated.After(history[right].updated) })
	start, end := 0, len(history)
	if page.Requested {
		start, end = page.Window(len(history))
	}
	result := make([]library.Item, end-start)
	for index, entry := range history[start:end] {
		result[index] = items[entry.position]
	}
	return result, len(history)
}

func RecentAdminProgress(values map[string]PlaybackState, items []library.Item, names map[string]string) []PlaybackView {
	titles := make(map[string]string, len(items))
	for _, item := range items {
		titles[item.ID] = item.Title
	}
	result := make([]PlaybackView, 0, len(values))
	for key, state := range values {
		profile, id, found := strings.Cut(key, ":")
		if !found || titles[id] == "" || state.Updated.IsZero() {
			continue
		}
		result = append(result, PlaybackView{names[profile], titles[id], ResumeAt(state.Seconds), state.Updated.Local().Format("Jan 2, 3:04 PM"), state.Updated})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].when.After(result[right].when) })
	return result[:min(20, len(result))]
}
