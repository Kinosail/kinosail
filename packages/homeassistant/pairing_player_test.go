package homeassistant

import (
	"bytes"
	"errors"
	"math"
	"net/http"
	"strings"
	"testing"
)

func TestPairing(t *testing.T) { //nolint:cyclop,gocognit,funlen // One matrix proves pairing validation, limits, and entropy failures.
	state := &testState{enabled: true, profile: Profile[testProfile]{Source: testProfile{"owner"}, ID: "owner", Owner: true}}
	integration := newTestIntegration(t, state)
	integration.pairs["expired"] = pairing[testProfile]{Expires: state.now}
	code, err := integration.offer(state.profile)
	if err != nil || code != "00000000" || len(integration.pairs) != 1 {
		t.Fatalf("offer = %q, %v, pairs %d", code, err, len(integration.pairs))
	}
	if token, pairErr := integration.pair(" 00000000 ", " Living room "); pairErr != nil || token != "token-owner" || state.created != 1 {
		t.Fatalf("pair = %q, %v", token, pairErr)
	}
	if _, pairErr := integration.pair("00000000", "Living room"); pairErr == nil || state.created != 1 {
		t.Fatalf("replayed pair = %v, creates %d", pairErr, state.created)
	}
	integration.config.Random = bytes.NewReader(make([]byte, 32))
	code, err = integration.offer(state.profile)
	if err != nil {
		t.Fatal(err)
	}
	if _, pairErr := integration.pair(code, strings.Repeat("n", 80)); pairErr != nil {
		t.Fatalf("pair name boundary = %v", pairErr)
	}
	for _, input := range []struct{ code, name string }{{"123", "name"}, {"1234567x", "name"}, {"12345678", ""}, {"12345678", strings.Repeat("n", 81)}} {
		if _, pairErr := integration.pair(input.code, input.name); pairErr == nil {
			t.Errorf("pair(%q,%q) accepted", input.code, input.name)
		}
	}
	integration.pairs["12345678"] = pairing[testProfile]{Profile: state.profile, Expires: state.now.Add(pairingTTL)}
	state.createErr = errors.New("create")
	if _, pairErr := integration.pair("12345678", "name"); !errors.Is(pairErr, state.createErr) {
		t.Fatalf("pair create = %v", pairErr)
	}

	integration.pairs = make(map[string]pairing[testProfile])
	for index := range maxPairs {
		integration.pairs[string(rune(index+1))] = pairing[testProfile]{Expires: state.now.Add(pairingTTL)}
	}
	if _, err = integration.offer(state.profile); err == nil || !strings.Contains(err.Error(), "too many") {
		t.Fatalf("offer limit = %v", err)
	}
	integration.pairs = make(map[string]pairing[testProfile])
	integration.config.Random = failingReader{errors.New("entropy")}
	if _, err = integration.offer(state.profile); err == nil {
		t.Fatal("offer accepted failed entropy")
	}
	integration.config.Random = bytes.NewReader(make([]byte, 128))
	integration.pairs["00000000"] = pairing[testProfile]{Expires: state.now.Add(pairingTTL)}
	if _, err = integration.offer(state.profile); err == nil || !strings.Contains(err.Error(), "could not create") {
		t.Fatalf("offer collisions = %v", err)
	}
}

func TestPairingPage(t *testing.T) {
	state := &testState{profile: Profile[testProfile]{Source: testProfile{"owner"}}}
	integration := newTestIntegration(t, state)
	notFound := false
	writeError := func(w http.ResponseWriter, _ *http.Request, message string, status int) {
		http.Error(w, message, status)
	}
	handler := integration.PairingHandler(func(w http.ResponseWriter, _ *http.Request, code string) error {
		_, _ = w.Write([]byte(code))
		return nil
	}, writeError, func(w http.ResponseWriter, _ *http.Request) { notFound = true; w.WriteHeader(http.StatusNotFound) })
	if got := response(handler, request(http.MethodGet, "/", nil)); got.Code != http.StatusNotFound || !notFound {
		t.Fatalf("disabled pairing = %d", got.Code)
	}
	state.enabled = true
	got := response(handler, request(http.MethodGet, "/", nil))
	if got.Code != http.StatusOK || got.Header().Get("Cache-Control") != "no-store" || got.Body.String() != "00000000" {
		t.Fatalf("pairing page = %d %q", got.Code, got.Body.String())
	}
	integration.config.Random = failingReader{errors.New("entropy")}
	if got = response(handler, request(http.MethodGet, "/", nil)); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("pairing offer error = %d", got.Code)
	}
	integration.config.Random = bytes.NewReader(make([]byte, 32))
	clear(integration.pairs)
	handler = integration.PairingHandler(func(http.ResponseWriter, *http.Request, string) error { return errors.New("render") }, writeError, http.NotFound)
	if got = response(handler, request(http.MethodGet, "/", nil)); got.Code != http.StatusInternalServerError {
		t.Fatalf("pairing render error = %d", got.Code)
	}
}

func TestPlayerAndCommandLifecycle(t *testing.T) { //nolint:cyclop,gocognit // Scores of 21 and 17 remain below the repository ceiling of 22 for one command lifecycle.
	state := &testState{enabled: true, profile: Profile[testProfile]{ID: "owner"}, items: map[string]Item{"item": {ID: "item"}}}
	integration := newTestIntegration(t, state)
	valid := Player{Name: " Player ", State: "playing", Position: 1, Duration: 2, Volume: .5}
	command, err := integration.updatePlayer("player", valid, "owner")
	if err != nil || command != nil || integration.players["player"].Name != "Player" {
		t.Fatalf("updatePlayer = %#v, %v", command, err)
	}
	events := 0
	integration.SetPublisher(func(profile, event, resource string) {
		if profile != "owner" || event != "home-assistant.command" || resource != "/api/v1/home-assistant/players/player" {
			t.Errorf("event = %q %q %q", profile, event, resource)
		}
		events++
	})
	if _, found, queueErr := integration.queueCommand("player", Command{Command: "pause"}); queueErr != nil || !found || events != 1 {
		t.Fatalf("queueCommand = %v, %v, events %d", found, queueErr, events)
	}
	integration.SetPublisher(nil)
	command, err = integration.updatePlayer("player", valid, "owner")
	if err != nil || command == nil || command.Command != "pause" {
		t.Fatalf("poll command = %#v, %v", command, err)
	}
	if _, found, err := integration.queueCommand("missing", Command{Command: "pause"}); err != nil || found {
		t.Fatalf("missing command = %v, %v", found, err)
	}
	integration.players["stale"] = playerRecord{Player: Player{ID: "stale"}, Seen: state.now.Add(-playerTTL)}
	if _, found, err := integration.queueCommand("stale", Command{Command: "pause"}); err != nil || !found {
		t.Fatalf("boundary command = %v, %v", found, err)
	}
	integration.players["expired"] = playerRecord{Player: Player{ID: "expired"}, Seen: state.now.Add(-playerTTL - 1)}
	if _, found, err := integration.queueCommand("expired", Command{Command: "pause"}); err != nil || found {
		t.Fatalf("expired command = %v, %v", found, err)
	}
	integration.players["player"] = playerRecord{Player: Player{ID: "player"}, Seen: state.now}
	if _, found, err := integration.queueCommand("player", Command{Command: "pause"}); err != nil || !found {
		t.Fatalf("nil publisher command = %v, %v", found, err)
	}
}

func TestPlayerValidationBoundaries(t *testing.T) { //nolint:cyclop // The score of 12 remains below the repository ceiling of 22 for the validation matrix.
	state := &testState{enabled: true, profile: Profile[testProfile]{ID: "owner"}, items: map[string]Item{"item": {ID: "item"}}}
	integration := newTestIntegration(t, state)
	valid := Player{Name: "Player", State: "playing", Position: 1, Duration: 2, Volume: .5}
	if _, err := integration.updatePlayer("player", valid, "owner"); err != nil {
		t.Fatal(err)
	}
	invalidPlayers := []Player{
		{},
		{Name: strings.Repeat("n", 81), State: "idle"},
		{Name: "n", State: "bad"},
		{Name: "n", State: "idle", Title: strings.Repeat("t", 257)},
		{Name: "n", State: "idle", ItemID: strings.Repeat("i", 129)},
		{Name: "n", State: "idle", Position: math.NaN()},
		{Name: "n", State: "idle", Duration: math.Inf(1)},
		{Name: "n", State: "idle", Volume: 2},
	}
	for _, player := range invalidPlayers {
		if _, err := integration.updatePlayer("bad", player, "owner"); err == nil {
			t.Errorf("invalid player accepted: %#v", player)
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1, 2} {
		if !invalidNumber(value, 1) {
			t.Errorf("invalidNumber(%v) = false", value)
		}
	}
	if invalidNumber(1, 1) {
		t.Fatal("valid boundary rejected")
	}
	boundaryItem := strings.Repeat("i", 128)
	state.items[boundaryItem] = Item{ID: boundaryItem}
	boundaryPlayer := Player{Name: strings.Repeat("n", 80), State: "idle", Title: strings.Repeat("t", 256), ItemID: boundaryItem, Position: 1e9, Duration: 1e9, Volume: 1}
	if _, err := integration.updatePlayer("boundary", boundaryPlayer, "owner"); err != nil {
		t.Fatalf("player boundaries = %v", err)
	}

	for _, command := range []Command{{Command: "bad"}, {Command: "seek", Position: -1}, {Command: "volume", Volume: 2}, {Command: "play_media"}, {Command: "play_media", ItemID: strings.Repeat("i", 129)}, {Command: "play_media", ItemID: "missing"}} {
		if _, _, err := integration.queueCommand("player", command); err == nil {
			t.Errorf("invalid command accepted: %#v", command)
		}
	}
	if _, _, err := integration.queueCommand("player", Command{Command: "play_media", ItemID: "item"}); err != nil {
		t.Fatalf("play_media = %v", err)
	}
	if err := integration.validateCommand(Command{Command: "play_media", ItemID: boundaryItem}); err != nil {
		t.Fatalf("play_media item boundary = %v", err)
	}
}

func TestPlayerCapacityAndPruning(t *testing.T) {
	state := &testState{enabled: true, profile: Profile[testProfile]{ID: "owner"}}
	integration := newTestIntegration(t, state)
	valid := Player{Name: "Player", State: "playing", Position: 1, Duration: 2, Volume: .5}
	integration.players = make(map[string]playerRecord)
	for index := range maxPlayers {
		id := strings.Repeat("x", index%63+1)
		integration.players[id+string(rune('A'+index%26))] = playerRecord{Player: Player{ID: "present"}, Seen: state.now}
	}
	if _, err := integration.updatePlayer("limit", valid, "owner"); !errors.Is(err, errPlayerLimit) {
		t.Fatalf("player limit = %v", err)
	}
	integration.players["old"] = playerRecord{Player: Player{ID: "old"}, Seen: state.now.Add(-playerTTL - 1)}
	integration.players["boundary"] = playerRecord{Player: Player{ID: "boundary"}, Seen: state.now.Add(-playerTTL)}
	players := integration.playersSnapshot()
	foundBoundary := false
	for _, player := range players {
		if player.ID == "old" {
			t.Fatal("playersSnapshot retained stale player")
		}
		foundBoundary = foundBoundary || player.ID == "boundary"
	}
	if !foundBoundary {
		t.Fatal("playersSnapshot pruned boundary player")
	}
}

func TestPlayerHTTPValidation(t *testing.T) {
	state := &testState{enabled: true, profile: Profile[testProfile]{ID: "owner"}}
	integration := newTestIntegration(t, state)
	for _, handler := range []http.HandlerFunc{integration.playerStateHTTP, integration.commandHTTP} {
		req := request(http.MethodPut, "/", strings.NewReader(`{}`))
		req.SetPathValue("id", "bad/id")
		got := response(handler, req)
		if got.Code != http.StatusBadRequest || !strings.Contains(got.Body.String(), "player ID") {
			t.Errorf("invalid ID = %d %s", got.Code, got.Body.String())
		}
	}
	req := request(http.MethodPut, "/", strings.NewReader(`{"name":"p","state":"idle","position":0,"duration":0,"volume":0}`))
	req.SetPathValue("id", "player")
	if got := response(http.HandlerFunc(integration.playerStateHTTP), req); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"command":null`) {
		t.Fatalf("player state = %d %s", got.Code, got.Body.String())
	}
	req = request(http.MethodPost, "/", strings.NewReader(`{"command":"pause"}`))
	req.SetPathValue("id", "player")
	if got := response(http.HandlerFunc(integration.commandHTTP), req); got.Code != http.StatusAccepted {
		t.Fatalf("command = %d %s", got.Code, got.Body.String())
	}
	req = request(http.MethodPut, "/", strings.NewReader(`{"name":"p","state":"idle","position":0,"duration":0,"volume":0}`))
	req.SetPathValue("id", "player")
	if got := response(http.HandlerFunc(integration.playerStateHTTP), req); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"command":"pause"`) {
		t.Fatalf("command poll = %d %s", got.Code, got.Body.String())
	}
}
