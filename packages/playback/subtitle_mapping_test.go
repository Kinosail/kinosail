package playback

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// WebVTT allows space/tab timing separators and CR, LF, or CRLF line endings:
// https://www.w3.org/TR/webvtt1/#webvtt-cue-timings
func TestServeSubtitleMapsTimingWithoutChangingTextOrSettings(t *testing.T) {
	for _, format := range []string{"vtt", "srt", "legacy-vtt"} {
		for eolIndex, eol := range []string{"\n", "\r\n", "\r"} {
			for spaceIndex, space := range []string{" ", "  ", "\t", " \t "} {
				t.Run(fmt.Sprintf("%s/eol%d/separator%d", format, eolIndex, spaceIndex), func(t *testing.T) {
					assertMappedSubtitleWhitespace(t, format, eol, space)
				})
			}
		}
	}
}

func assertMappedSubtitleWhitespace(t *testing.T, format, eol, space string) {
	t.Helper()
	extension := strings.TrimPrefix(format, "legacy-")
	settings := ""
	if extension == "vtt" {
		settings = "position:10%,line-left align:start"
	}
	input := "1\n00:00:16.000" + space + "-->" + space + "00:00:18.000" + space + settings + "\nHello, world\n\n2\n00:00:06.000" + space + "-->" + space + "00:00:08.000\nInside skipped segment\n"
	if format != "vtt" {
		input = strings.ReplaceAll(input, ".000", ",000")
	}
	if extension == "vtt" {
		input = "WEBVTT\n\n" + input
	}
	input = strings.ReplaceAll(input, "\n", eol)
	path := filepath.Join(t.TempDir(), "captions."+extension)
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	timeline := Timeline{SourceDuration: 20, Duration: 10, Omitted: []Range{{Start: 5, End: 15}}}
	dependencies := SubtitleDeliveryDependencies{
		ParseTimeline: func(string) (Timeline, error) { return timeline, nil },
		NotFound: func(http.ResponseWriter, *http.Request) {
			t.Fatal("valid subtitle was not found")
		},
		Error: func(http.ResponseWriter, *http.Request, string, int) {
			t.Fatal("valid subtitle caused a delivery error")
		},
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle/item?playbackToken=synthetic", nil)
	ServeSubtitle(response, request, path, nil, dependencies)
	assertSubtitleResponse(t, response, strings.TrimSpace("00:00:06.000 --> 00:00:08.000 "+settings), "Hello, world")
	if strings.Contains(response.Body.String(), "Inside skipped segment") {
		t.Fatal("a cue inside the omitted segment was delivered")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != input {
		t.Fatal("subtitle delivery modified its source")
	}
}
