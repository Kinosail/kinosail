package homeassistant

import "testing"

func TestPairingReloadsCurrentOwnerBeforeIssuingKey(t *testing.T) {
	state := &testState{enabled: true, profile: Profile[testProfile]{Source: testProfile{"owner"}, ID: "owner", Owner: true}}
	integration := newTestIntegration(t, state)
	code, err := integration.offer(state.profile)
	if err != nil {
		t.Fatal(err)
	}
	state.profile.Owner = false
	if token, err := integration.pair(code, "Home"); err == nil || token != "" || state.created != 0 {
		t.Fatalf("demoted Owner issued key: %q %v", token, err)
	}
}

func TestPlayerCannotConsumeAnotherProfilesCommand(t *testing.T) {
	state := &testState{enabled: true}
	integration := newTestIntegration(t, state)
	player := Player{Name: "TV", State: "idle"}
	if _, err := integration.updatePlayer("tv", player, "first"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := integration.queueCommand("tv", Command{Command: "pause"}); err != nil {
		t.Fatal(err)
	}
	if command, err := integration.updatePlayer("tv", player, "second"); err == nil || command != nil || integration.players["tv"].Profile != "first" {
		t.Fatal("second profile reassigned player or consumed command")
	}
	if command, err := integration.updatePlayer("tv", player, "first"); err != nil || command == nil || command.Command != "pause" {
		t.Fatal("rejected update altered the pending command")
	}
}
