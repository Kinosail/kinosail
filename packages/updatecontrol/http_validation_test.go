package updatecontrol

import "testing"

func TestParseModeValidatesPreferenceAndAllowedPeers(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		body     string
		required bool
		allowed  []string
		want     bool
		wantErr  bool
	}{
		"optional missing": {body: "name=Owner", allowed: []string{"name"}, want: true},
		"required missing": {body: "name=Owner", required: true, allowed: []string{"name"}, wantErr: true},
		"manual":           {body: "updateMode=manual&name=Owner", allowed: []string{"name"}},
		"automatic":        {body: "updateMode=automatic&name=Owner", allowed: []string{"name"}, want: true},
		"unknown peer":     {body: "updateMode=automatic&extra=true", allowed: []string{"name"}, wantErr: true},
		"unknown mode":     {body: "updateMode=sometimes", wantErr: true},
		"duplicate":        {body: "updateMode=manual&updateMode=automatic", wantErr: true},
		"malformed":        {body: "updateMode=%zz", wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			request := formRequest(t, "/setup", test.body)
			got, err := ParseMode(request, test.required, test.allowed...)
			if got != test.want || (err != nil) != test.wantErr {
				t.Fatalf("ParseMode = %t, %v", got, err)
			}
		})
	}
}

func TestUpdateStatusTextCoversEveryState(t *testing.T) {
	t.Parallel()
	for state, expected := range map[string]string{
		"available": "v1.2.3 is available", "current": "Kinosail is current", "unavailable": "The last check could not reach GitHub",
		"container-managed": "Update this installation by deploying a newer Kinosail container.",
		"no-release":        "No public Kinosail release is available yet", "not-checked": "No update check has run",
	} {
		if actual := StatusText(Status{State: state, LatestVersion: "v1.2.3"}); actual != expected {
			t.Fatalf("state %q text = %q, want %q", state, actual, expected)
		}
	}
	for state, expected := range map[string]string{
		"requested": "The installed update manager has received the update request.", "checking": "The installed update manager is checking.",
		"available": "The installed update manager is available.", "installing": "The installed update manager is installing.",
		"current": "The installed update manager reports that Kinosail is current.", "failed": "The installed update manager reports: failed update",
		"rolled-back": "The installed update manager reports: failed update", "unknown": "No installed update adapter has reported yet.",
	} {
		if actual := ManagerStatusText(View{Status: state, Message: "failed update"}); actual != expected {
			t.Fatalf("manager state %q text = %q, want %q", state, actual, expected)
		}
	}
}
