package server

import (
	"sync"
	"time"
)

type subtitleAutomationStatus struct {
	LastRun   string `json:"lastRun,omitempty"`
	NextRun   string `json:"nextRun,omitempty"`
	Attempted int    `json:"attempted"`
	Added     int    `json:"added"`
	Upgraded  int    `json:"upgraded"`
	Failed    int    `json:"failed"`
	Running   bool   `json:"running"`
	Started   string `json:"started,omitempty"`
}

type subtitleAutomationClock struct {
	mu     sync.Mutex
	status subtitleAutomationStatus
}

func (manager *subtitleManager) noteAutomation(result subtitleMaintenanceResult, ran time.Time) {
	manager.automation.mu.Lock()
	defer manager.automation.mu.Unlock()
	manager.automation.status = subtitleAutomationStatus{
		LastRun: ran.UTC().Format(time.RFC3339), NextRun: ran.Add(subtitleAutomationInterval).UTC().Format(time.RFC3339),
		Attempted: result.Attempted, Added: result.Added, Upgraded: result.Upgraded, Failed: result.Failed,
	}
}

func (manager *subtitleManager) automationStatus() subtitleAutomationStatus {
	manager.automation.mu.Lock()
	defer manager.automation.mu.Unlock()
	return manager.automation.status
}
