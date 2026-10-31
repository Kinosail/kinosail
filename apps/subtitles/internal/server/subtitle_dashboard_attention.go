package server

import (
	"net/http"
	"sort"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type subtitleWork struct {
	Label string `json:"label"`
	Title string `json:"title,omitempty"`
	ID    string `json:"id,omitempty"`
}

var subtitleWorkLabels = map[string]string{
	"maintain":     "Checking up to 10 subtitles",
	"fetch-wanted": "Finding wanted subtitles",
	"fetch":        "Finding a subtitle",
	"apply":        "Saving subtitle changes",
	"restore":      "Restoring a subtitle",
	"replacement":  "Changing upgrade protection",
	"audio":        "Checking timing against the audio",
}

func (manager *subtitleManager) timingUnchecked(item library.Item, languages []string) bool {
	if manager.provider == nil || manager.provider.ledger == nil {
		return false
	}
	for _, language := range languages {
		record, found, err := manager.provider.ledger.record(subtitleRecordKey(item.ID, language))
		if err != nil || !found || !record.Managed || record.Frozen || record.TimingEvidence != "unverified" {
			continue
		}
		for _, path := range item.Subtitles {
			if subtitleLanguageFromPath(item.Path, path) == language {
				return true
			}
		}
	}
	return false
}

func (manager *subtitleManager) activity(request *http.Request) []subtitleWork {
	activity := []subtitleWork{}
	if manager.automationStatus().Running {
		activity = append(activity, subtitleWork{Label: "Automatic check"})
	}
	if manager.operations == nil {
		return activity
	}
	for _, receipt := range manager.operations.running() {
		entry := subtitleWork{Label: subtitleWorkLabels[receipt.Action], ID: receipt.Item}
		if receipt.Item != "" && manager.index != nil {
			if item, found := manager.index.VisibleItem(request, receipt.Item); found {
				entry.Title = subtitleDashboardIdentity(item).Title
			}
		}
		activity = append(activity, entry)
	}
	return activity
}

func (operations *subtitleOperations) running() []subtitleOperationReceipt {
	operations.mu.Lock()
	defer operations.mu.Unlock()
	running := []subtitleOperationRecord{}
	for _, record := range operations.records {
		if record.State == "running" {
			running = append(running, record)
		}
	}
	sort.Slice(running, func(left, right int) bool {
		if running[left].Created != running[right].Created {
			return running[left].Created < running[right].Created
		}
		return running[left].ID < running[right].ID
	})
	receipts := make([]subtitleOperationReceipt, 0, len(running))
	for _, record := range running {
		receipts = append(receipts, record.subtitleOperationReceipt)
	}
	return receipts
}

func (manager *subtitleManager) noteAutomationStart(started time.Time) {
	manager.automation.mu.Lock()
	defer manager.automation.mu.Unlock()
	manager.automation.status.Running = true
	manager.automation.status.Started = started.UTC().Format(time.RFC3339)
}
