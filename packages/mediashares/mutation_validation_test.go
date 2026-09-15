package mediashares

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

func indexedSecret(index int) string {
	value := make([]byte, 32)
	binary.BigEndian.PutUint64(value[24:], uint64(index)+1) //nolint:gosec // Test indexes are bounded by small fixtures.
	return base64.RawURLEncoding.EncodeToString(value)
}

func indexedHash(index int) string {
	return fmt.Sprintf("%064x", index+1)
}

func TestStateValidatorsEnforceEveryBoundary(t *testing.T) { //nolint:cyclop,funlen,gocognit // The table isolates every persisted field boundary.
	secret, hash := indexedSecret(0), indexedHash(0)
	base := Share{ID: secret, ItemIDs: []string{"item"}, ClaimHash: hash, ExpiresAt: 1, MaxDevices: 1, RightsAcknowledged: true}
	for name, test := range map[string]struct {
		id    string
		share Share
		valid bool
	}{
		"valid":           {secret, base, true},
		"wrong ID":        {indexedSecret(1), base, false},
		"invalid secret":  {"invalid", Share{ID: "invalid", ClaimHash: hash, ExpiresAt: 1, MaxDevices: 1, RightsAcknowledged: true}, false},
		"invalid hash":    {secret, Share{ID: secret, ClaimHash: "invalid", ExpiresAt: 1, MaxDevices: 1, RightsAcknowledged: true}, false},
		"zero expiry":     {secret, Share{ID: secret, ClaimHash: hash, MaxDevices: 1, RightsAcknowledged: true}, false},
		"minimum devices": {secret, base, true},
		"maximum devices": {secret, Share{ID: secret, ClaimHash: hash, ExpiresAt: 1, MaxDevices: 8, RightsAcknowledged: true}, true},
		"low devices":     {secret, Share{ID: secret, ClaimHash: hash, ExpiresAt: 1, MaxDevices: 0, RightsAcknowledged: true}, false},
		"high devices":    {secret, Share{ID: secret, ClaimHash: hash, ExpiresAt: 1, MaxDevices: 9, RightsAcknowledged: true}, false},
		"missing rights":  {secret, Share{ID: secret, ClaimHash: hash, ExpiresAt: 1, MaxDevices: 1}, false},
	} {
		if validShareFields(test.id, test.share) != test.valid {
			t.Errorf("%s share validity = %v", name, !test.valid)
		}
	}

	hundred := make([]string, 100)
	for index := range hundred {
		hundred[index] = fmt.Sprintf("item-%d", index)
	}
	for name, test := range map[string]struct {
		items []string
		valid bool
	}{
		"one": {[]string{"x"}, true}, "hundred": {hundred, true}, "empty": {nil, false},
		"too many": {append(slices.Clone(hundred), "extra"), false}, "empty ID": {[]string{""}, false},
		"256 bytes": {[]string{strings.Repeat("x", 256)}, true}, "257 bytes": {[]string{strings.Repeat("x", 257)}, false},
		"duplicate": {[]string{"x", "x"}, false},
	} {
		if validItemIDs(test.items) != test.valid {
			t.Errorf("%s item validity = %v", name, !test.valid)
		}
	}

	shares := map[string]Share{secret: {ID: secret, ExpiresAt: 2}}
	for name, test := range map[string]struct {
		key     string
		session Session
		valid   bool
	}{
		"valid":           {hash, Session{ShareID: secret, ExpiresAt: 1, Device: strings.Repeat("x", 80)}, true},
		"invalid key":     {"invalid", Session{ShareID: secret, ExpiresAt: 1}, false},
		"missing share":   {hash, Session{ShareID: "missing", ExpiresAt: 1}, false},
		"zero expiry":     {hash, Session{ShareID: secret}, false},
		"at share expiry": {hash, Session{ShareID: secret, ExpiresAt: 2}, true},
		"after share":     {hash, Session{ShareID: secret, ExpiresAt: 3}, false},
		"long device":     {hash, Session{ShareID: secret, ExpiresAt: 1, Device: strings.Repeat("x", 81)}, false},
	} {
		if validSession(shares, test.key, test.session) != test.valid {
			t.Errorf("%s session validity = %v", name, !test.valid)
		}
	}

	for name, test := range map[string]struct {
		value string
		valid bool
	}{
		"secret 31":        {base64.RawURLEncoding.EncodeToString(make([]byte, 31)), false},
		"secret 32":        {base64.RawURLEncoding.EncodeToString(make([]byte, 32)), true},
		"secret 33":        {base64.RawURLEncoding.EncodeToString(make([]byte, 33)), false},
		"secret malformed": {"invalid", false},
	} {
		if ValidSecret(test.value) != test.valid {
			t.Errorf("%s validity = %v", name, !test.valid)
		}
	}
	for length, valid := range map[int]bool{62: false, 64: true, 66: false} {
		if ValidHash(strings.Repeat("0", length)) != valid {
			t.Errorf("hash length %d validity = %v", length, !valid)
		}
	}
	if ValidHash(strings.Repeat("z", 64)) {
		t.Fatal("malformed hash was accepted")
	}
}

func TestStateCollectionAndPruneBoundaries(t *testing.T) { //nolint:cyclop,funlen // Exact collection limits need populated state.
	shares := make(map[string]Share, 257)
	for index := range 256 {
		id := indexedSecret(index)
		shares[id] = Share{ID: id, ItemIDs: []string{"item"}, ClaimHash: indexedHash(index), ExpiresAt: 2, MaxDevices: 1, RightsAcknowledged: true}
	}
	state := State{Shares: shares, Sessions: map[string]Session{}}
	if !ValidState(state) {
		t.Fatal("maximum share count was rejected")
	}
	id := indexedSecret(256)
	state.Shares[id] = Share{ID: id, ItemIDs: []string{"item"}, ClaimHash: indexedHash(256), ExpiresAt: 2, MaxDevices: 1, RightsAcknowledged: true}
	if ValidState(state) {
		t.Fatal("share count above maximum was accepted")
	}

	secret := indexedSecret(0)
	state = State{Shares: map[string]Share{secret: {ID: secret, ItemIDs: []string{"item"}, ClaimHash: indexedHash(0), ExpiresAt: 2, MaxDevices: 1, RightsAcknowledged: true}}, Sessions: make(map[string]Session, 2049)}
	for index := range 2048 {
		state.Sessions[indexedHash(index)] = Session{ShareID: secret, ExpiresAt: 1}
	}
	if !ValidState(state) {
		t.Fatal("maximum session count was rejected")
	}
	state.Sessions[indexedHash(2048)] = Session{ShareID: secret, ExpiresAt: 1}
	if ValidState(state) {
		t.Fatal("session count above maximum was accepted")
	}

	keep, expire := indexedSecret(1), indexedSecret(2)
	state = State{
		Shares: map[string]Share{keep: {ID: keep, ExpiresAt: 2}, expire: {ID: expire, ExpiresAt: 1}},
		Sessions: map[string]Session{
			indexedHash(1): {ShareID: keep, ExpiresAt: 2}, indexedHash(2): {ShareID: keep, ExpiresAt: 1}, indexedHash(3): {ShareID: "missing", ExpiresAt: 2},
		},
	}
	Prune(&state, 1)
	if len(state.Shares) != 1 || state.Shares[keep].ID != keep || len(state.Sessions) != 1 || state.Sessions[indexedHash(1)].ShareID != keep {
		t.Fatalf("pruned state = %#v", state)
	}
}

func TestRequestValidatorsEnforceEveryBoundary(t *testing.T) { //nolint:funlen // The table isolates every request boundary.
	hundred := make([]string, 100)
	for index := range hundred {
		hundred[index] = fmt.Sprintf("item-%d", index)
	}
	for name, test := range map[string]struct {
		items        []string
		lifetime     time.Duration
		devices      int
		acknowledged bool
		valid        bool
	}{
		"minimum": {[]string{"item"}, time.Minute, 1, true, true}, "maximum": {hundred, 24 * time.Hour, 8, true, true},
		"empty": {nil, time.Minute, 1, true, false}, "too many": {append(slices.Clone(hundred), "extra"), time.Minute, 1, true, false},
		"short": {[]string{"item"}, time.Minute - 1, 1, true, false}, "long": {[]string{"item"}, 24*time.Hour + 1, 1, true, false},
		"device low": {[]string{"item"}, time.Minute, 0, true, false}, "device high": {[]string{"item"}, time.Minute, 9, true, false},
		"rights": {[]string{"item"}, time.Minute, 1, false, false},
	} {
		if validCreatePolicy(test.items, test.lifetime, test.devices, test.acknowledged) != test.valid {
			t.Errorf("%s creation validity = %v", name, !test.valid)
		}
	}
	for name, test := range map[string]struct {
		token, device string
		valid         bool
	}{
		"minimum": {strings.Repeat("x", 32), strings.Repeat("x", 80), true},
		"maximum": {strings.Repeat("x", 256), "", true}, "short": {strings.Repeat("x", 31), "", false},
		"long": {strings.Repeat("x", 257), "", false}, "device": {strings.Repeat("x", 32), strings.Repeat("x", 81), false},
	} {
		if validClaimInput(test.token, test.device) != test.valid {
			t.Errorf("%s claim validity = %v", name, !test.valid)
		}
	}
}

func TestHTTPValidatorsEnforceEveryBoundary(t *testing.T) { //nolint:cyclop,funlen,gocognit // The table isolates form and template token boundaries.
	for name, test := range map[string]struct {
		product string
		asset   string
		hasCSRF bool
		valid   bool
	}{
		"valid": {"Player", "75", true, true}, "missing product": {"", "75", true, false},
		"missing asset": {"Player", "", true, false}, "missing CSRF": {"Player", "75", false, false},
	} {
		if validViewConfiguration(test.product, test.asset, test.hasCSRF) != test.valid {
			t.Errorf("%s view configuration validity = %v", name, !test.valid)
		}
	}
	if !validHTTPDependencies(true, true, true, true, true, true) || validHTTPDependencies(true, true, true, true, true) || validHTTPDependencies(true, true, true, true, true, true, true) {
		t.Fatal("HTTP dependency count was not enforced")
	}
	for missing := range 6 {
		present := []bool{true, true, true, true, true, true}
		present[missing] = false
		if validHTTPDependencies(present...) {
			t.Errorf("missing HTTP dependency %d was accepted", missing)
		}
	}
	for name, test := range map[string]struct {
		length int64
		query  string
		valid  bool
	}{
		"exact": {64 << 10, "", true}, "over": {64<<10 + 1, "", false}, "query": {0, "x=1", false},
	} {
		if validFormEnvelope(test.length, test.query) != test.valid {
			t.Errorf("%s form envelope validity = %v", name, !test.valid)
		}
	}
	for value, valid := range map[int64]bool{59: false, 60: true, 86400: true, 86401: false} {
		if validLifetimeSeconds(value) != valid {
			t.Errorf("lifetime %d validity = %v", value, !valid)
		}
	}
	for name, test := range map[string]struct {
		value   string
		maximum int
		valid   bool
	}{
		"one": {"A", 1, true}, "exact": {strings.Repeat("A", 32), 32, true}, "empty": {"", 1, false},
		"long": {"AA", 1, false}, "space": {"A Z", 3, true}, "before upper": {"@", 1, false},
		"after upper": {"[", 1, false}, "before lower": {"`", 1, false}, "lower bounds": {"az", 2, true}, "after lower": {"{", 1, false},
	} {
		if safeViewToken(test.value, test.maximum) != test.valid {
			t.Errorf("%s view token validity = %v", name, !test.valid)
		}
	}
	for value, valid := range map[string]bool{"0": true, "9": true, "12345678": true, "": false, "123456789": false, "/": false, ":": false} {
		if safeAssetVersion(value) != valid {
			t.Errorf("asset %q validity = %v", value, !valid)
		}
	}
	for _, key := range []string{"itemIds", "expires", "devices", "rightsAcknowledged", "_csrf"} {
		if !validCreateField(key, []string{"x"}) {
			t.Errorf("allowed field %q was rejected", key)
		}
	}
	if !validCreateField("itemIds", []string{strings.Repeat("x", 256)}) {
		t.Fatal("maximum field value was rejected")
	}
	for name, values := range map[string][]string{"unknown": {"x"}, "empty": {}, "long": {strings.Repeat("x", 257)}} {
		key := "itemIds"
		if name == "unknown" {
			key = "unknown"
		}
		if validCreateField(key, values) {
			t.Errorf("%s field was accepted", name)
		}
	}
	base := url.Values{"itemIds": {"item"}, "expires": {"3600"}, "devices": {"1"}, "rightsAcknowledged": {"true"}}
	if !validCreateForm(base) {
		t.Fatal("valid create form was rejected")
	}
	items := make([]string, 100)
	for index := range items {
		items[index] = fmt.Sprintf("item-%d", index)
	}
	maximum := url.Values{"itemIds": items, "expires": {"3600"}, "devices": {"1"}, "rightsAcknowledged": {"true"}}
	if !validCreateForm(maximum) {
		t.Fatal("maximum item form was rejected")
	}
	maximum["itemIds"] = append(maximum["itemIds"], "extra")
	if validCreateForm(maximum) {
		t.Fatal("item form above maximum was accepted")
	}
	for _, key := range []string{"itemIds", "expires", "devices", "rightsAcknowledged"} {
		form := url.Values{"itemIds": {"item"}, "expires": {"3600"}, "devices": {"1"}, "rightsAcknowledged": {"true"}}
		delete(form, key)
		if validCreateForm(form) {
			t.Errorf("missing %s was accepted", key)
		}
	}
}
