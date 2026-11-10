package server_test

import (
	"encoding/base64"
	"net/http"
	"os"
	"strings"
	"testing"
)

const hearingImpairedFixture = "1\n00:00:01,000 --> 00:00:02,000\n[door slams]\n\n2\n00:00:03,000 --> 00:00:04,000\nJOHN: Hello there. (sighs)\n\n3\n00:00:05,000 --> 00:00:06,000\n- [laughing] You came.\n- MARY: I did.\n\n4\n00:00:07,000 --> 00:00:08,000\n♪ ♪\n\n5\n00:00:09,000 --> 00:00:10,000\n<i>(phone\nringing)</i>\n\n6\n00:00:11,000 --> 00:00:12,000\nMeet at 10:30: sharp.\n"

func TestSubtitleInspectorRemovesHearingImpairedDescriptionsOnRequest(t *testing.T) {
	for _, sample := range []struct {
		name, option, want string
	}{
		{"requested", `,"removeHearingImpaired":true`, "1\n00:00:03,000 --> 00:00:04,000\nHello there.\n\n2\n00:00:05,000 --> 00:00:06,000\n- You came.\n- I did.\n\n3\n00:00:11,000 --> 00:00:12,000\nMeet at 10:30: sharp.\n\n"},
		{"default", "", hearingImpairedFixture + "\n"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			handler, base, target := subtitleInspectorFixture(t, "")
			input := `{"language":"en","fingerprint":"missing","data":"` + base64.StdEncoding.EncodeToString([]byte(hearingImpairedFixture)) + `"` + sample.option + `}`
			preview := requestJSON(t, handler, http.MethodPost, base+"/preview", input)
			if preview.Code != http.StatusOK || strings.Contains(preview.Body.String(), "Removed hearing-impaired descriptions") != (sample.option != "") {
				t.Fatalf("preview = %d %s", preview.Code, preview.Body.String())
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatal("preview changed the sidecar")
			}
			saved := requestJSON(t, handler, http.MethodPost, base+"/apply", input)
			if saved.Code != http.StatusOK {
				t.Fatalf("save = %d %s", saved.Code, saved.Body.String())
			}
			data, err := os.ReadFile(target)
			if err != nil || string(data) != sample.want {
				t.Fatalf("saved subtitle = %q, %v; want %q", data, err, sample.want)
			}
		})
	}
}

func TestSubtitleInspectorRejectsAFileThatIsOnlyHearingImpairedDescriptions(t *testing.T) {
	handler, base, target := subtitleInspectorFixture(t, "")
	only := "1\n00:00:01,000 --> 00:00:02,000\n[door slams]\n"
	input := `{"language":"en","fingerprint":"missing","removeHearingImpaired":true,"data":"` + base64.StdEncoding.EncodeToString([]byte(only)) + `"}`
	saved := requestJSON(t, handler, http.MethodPost, base+"/apply", input)
	if saved.Code != http.StatusBadRequest {
		t.Fatalf("save = %d %s", saved.Code, saved.Body.String())
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("rejected edit changed the sidecar")
	}
}
