package casting

import (
	"math"
	"strings"
	"testing"
)

func TestStartRejectsInvalidInputs(t *testing.T) {
	for _, input := range []Start{
		{},
		{Protocol: "roku", Position: 0},
		{Protocol: "google-cast", DeviceID: strings.Repeat("a", 32)},
		{Protocol: "dlna"},
		{Protocol: "dlna", DeviceID: "http://192.168.1.2/control"},
		{Protocol: "google-cast", Position: -1},
		{Protocol: "google-cast", Position: math.NaN()},
		{Protocol: "google-cast", Position: math.Inf(1)},
		{Protocol: "google-cast", Position: 31_536_001},
		{Protocol: "google-cast", PlaybackToken: strings.Repeat("x", 8193)},
	} {
		if input.Validate("movie") == nil {
			t.Errorf("accepted %#v", input)
		}
	}
	for _, id := range []string{"", "../movie", strings.Repeat("x", 129), "movie\n"} {
		if (Start{Protocol: "google-cast"}).Validate(id) == nil {
			t.Errorf("accepted item %q", id)
		}
	}
	if (Start{Protocol: "google-cast"}).Validate("movie") != nil {
		t.Fatal("valid Google Cast rejected")
	}
	if (Start{Protocol: "dlna", DeviceID: strings.Repeat("a", 32), Position: 12}).Validate("movie") != nil {
		t.Fatal("valid DLNA rejected")
	}
}

func TestCommandsRejectMissingConflictingAndUnboundedValues(t *testing.T) {
	negative, valid := float64(-1), float64(12)
	for _, command := range []Command{{}, {Action: "reboot"}, {Action: "seek"}, {Action: "seek", Position: &negative}, {Action: "pause", Position: &valid}, {Action: "stop", Position: &valid}} {
		if command.Validate() == nil {
			t.Errorf("accepted %#v", command)
		}
	}
	for _, command := range []Command{{Action: "play"}, {Action: "pause"}, {Action: "stop"}, {Action: "seek", Position: &valid}} {
		if command.Validate() != nil {
			t.Errorf("rejected %#v", command)
		}
	}
}
