package dashboard

import (
	"context"
	"testing"
)

func TestURLUpdateTracksOnlyAnInheritedHealthAddress(t *testing.T) {
	for _, test := range []struct {
		name       string
		healthURL  string
		wantHealth string
	}{
		{name: "inherited", healthURL: "https://old.example.test", wantHealth: "https://new.example.test"},
		{name: "explicit", healthURL: "https://health.example.test", wantHealth: "https://health.example.test"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := testApp(1, "https://old.example.test", true)
			app.HealthURL = test.healthURL
			service, _ := serviceForTest(t, testBoard(app))
			updatedURL := "https://new.example.test"
			updated, _, err := service.Update(context.Background(), app.ID, UpdateInput{URL: &updatedURL, ExpectedVersion: 1}, "Owner")
			if err != nil {
				t.Fatal(err)
			}
			if updated.URL != updatedURL || updated.HealthURL != test.wantHealth {
				t.Fatalf("updated addresses = %q, %q", updated.URL, updated.HealthURL)
			}
		})
	}
}
