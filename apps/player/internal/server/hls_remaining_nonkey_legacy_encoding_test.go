package server

import (
	"bytes"
	"encoding/json"
	"testing"
)

// Canonical metadata written before presentation origins must retain its bytes:
// refill snapshots compare marshaled timelines with the certified original.
func TestRemainingNonKeyLegacyEncodingRetainsBytes(t *testing.T) {
	expected := []byte(`{"Policy":"owned-source-policy","Strategy":"h264-idr-keys-1","Numerator":1,"Denominator":1000,"TimeBase":0.001,"Keys":[{"PTS":12000,"DTS":11917},{"PTS":14000,"DTS":13917}],"End":16,"Clock":0}`)
	var timeline copiedHLSTimeline
	if json.Unmarshal(expected, &timeline) != nil || !validCopiedHLSTimeline(&timeline) {
		t.Fatal("nonkey legacy canonical metadata rejected")
	}
	actual, err := json.Marshal(&timeline)
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("nonkey legacy timeline bytes changed during decode/reencode")
	}
	if timeline.point(0) != 12 || timeline.Clock == nil || *timeline.Clock != 0 {
		t.Fatal("nonkey legacy metadata coordinates changed")
	}
}
