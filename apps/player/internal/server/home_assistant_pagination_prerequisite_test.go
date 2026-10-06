package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestHomeAssistantPaginationFixtureDisablesAutomaticUpdates(t *testing.T) {
	fixture := newQ12PaginationFixture(t)
	status, body := fixture.get(t, fixture.owner, "/api/v1/settings")
	if status != http.StatusOK {
		t.Fatalf("fixture settings status = %d", status)
	}
	var settings struct {
		Updates struct {
			Automatic *bool `json:"automatic"`
		} `json:"updates"`
	}
	if err := json.Unmarshal(body, &settings); err != nil {
		t.Fatal("fixture settings response is not JSON")
	}
	if settings.Updates.Automatic == nil || *settings.Updates.Automatic {
		t.Fatal("fixture setup must keep automatic update checks disabled")
	}
}
