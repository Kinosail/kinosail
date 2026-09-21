package casting

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func controlledRenderer(t *testing.T, transport roundTripFunc) (*Renderers, string) {
	t.Helper()
	renderers := NewRenderers()
	id := strings.Repeat("b", 32)
	renderers.devices[id] = Device{ID: id, control: "http://192.168.1.20/control", service: transportType, expires: time.Now().Add(time.Minute)}
	renderers.client = &http.Client{Transport: transport}
	return renderers, id
}

func soapResult(name, content string) string {
	return `<Envelope><Body><` + name + `Response>` + content + `</` + name + `Response></Body></Envelope>`
}

func TestRendererPollingNormalizesStateAndRedactsSource(t *testing.T) {
	for _, tc := range []struct{ remote, state string }{{"PLAYING", "playing"}, {"PAUSED_PLAYBACK", "paused"}, {"STOPPED", "stopped"}, {"NO_MEDIA_PRESENT", "stopped"}, {"TRANSITIONING", "buffering"}} {
		t.Run(tc.remote, func(t *testing.T) {
			calls := 0
			r, id := controlledRenderer(t, func(request *http.Request) (*http.Response, error) {
				calls++
				if strings.Contains(request.Header.Get("SOAPAction"), "#GetTransportInfo") {
					return reply(soapResult("GetTransportInfo", "<CurrentTransportState>"+tc.remote+"</CurrentTransportState>")), nil
				}
				return reply(soapResult("GetPositionInfo", `<TrackURI>http://192.168.1.1/cast/item?ticket=private</TrackURI><RelTime>01:02:03.5</RelTime><TrackDuration>02:00:00</TrackDuration>`)), nil
			})
			now := time.Now()
			r.now = func() time.Time { return now }
			status, err := r.Status(t.Context(), id)
			if err != nil || status.State != tc.state || status.Position != 3723.5 || status.Duration != 7200 || status.Source != "http://192.168.1.1/cast/item" || calls != 2 {
				t.Fatalf("status=%+v err=%v calls=%d", status, err, calls)
			}
			if !r.devices[id].expires.Equal(now.Add(10 * time.Minute)) {
				t.Fatal("successful polling did not renew device")
			}
		})
	}
}

func TestRendererPollingRejectsInvalidResponsesWithoutRenewal(t *testing.T) {
	validState := soapResult("GetTransportInfo", `<CurrentTransportState>PLAYING</CurrentTransportState>`)
	validPosition := soapResult("GetPositionInfo", `<TrackURI>http://192.168.1.1/media</TrackURI><RelTime>00:00:01</RelTime><TrackDuration>00:01:00</TrackDuration>`)
	for _, tc := range []struct {
		name, state, position string
		failCall              int
	}{
		{"transport unavailable", validState, validPosition, 1},
		{"malformed state", "<", validPosition, 0},
		{"unknown state", soapResult("GetTransportInfo", `<CurrentTransportState>unknown</CurrentTransportState>`), validPosition, 0},
		{"position unavailable", validState, validPosition, 2},
		{"malformed position", validState, "<", 0},
		{"invalid elapsed", validState, strings.Replace(validPosition, "00:00:01", "bad", 1), 0},
		{"invalid duration", validState, strings.Replace(validPosition, "00:01:00", "bad", 1), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertRejectedRendererPolling(t, tc.state, tc.position, tc.failCall)
		})
	}
	r, _ := controlledRenderer(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("unknown renderer reached network")
		return nil, nil
	})
	if _, err := r.Status(t.Context(), strings.Repeat("c", 32)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unknown renderer: %v", err)
	}
}

func TestRendererLoadStopsAfterFailedResume(t *testing.T) {
	for _, failure := range []string{"SetAVTransportURI", "Play", "Seek", ""} {
		t.Run(failure, func(t *testing.T) {
			assertRendererLoadFailure(t, failure)
		})
	}
}

func TestRendererCommandsUseFixedActions(t *testing.T) {
	for _, tc := range []struct{ command, action string }{{"play", "Play"}, {"pause", "Pause"}, {"stop", "Stop"}, {"seek", "Seek"}} {
		t.Run(tc.command, func(t *testing.T) {
			calls := 0
			r, id := controlledRenderer(t, func(request *http.Request) (*http.Response, error) {
				calls++
				if request.Header.Get("SOAPAction") != `"`+transportType+"#"+tc.action+`"` {
					t.Fatal("wrong SOAP action")
				}
				return reply("<ok/>"), nil
			})
			command := Command{Action: tc.command}
			position := 62.0
			if tc.command == "seek" {
				command.Position = &position
			}
			if err := r.Command(t.Context(), id, command); err != nil || calls != 1 {
				t.Fatalf("command=%v calls=%d", err, calls)
			}
		})
	}
}

func assertRejectedRendererPolling(t *testing.T, state, position string, failCall int) {
	t.Helper()
	calls := 0
	r, id := controlledRenderer(t, func(*http.Request) (*http.Response, error) {
		calls++
		if calls == failCall {
			return nil, errors.New("offline")
		}
		if calls == 1 {
			return reply(state), nil
		}
		return reply(position), nil
	})
	expiry := r.devices[id].expires
	if _, err := r.Status(t.Context(), id); err == nil {
		t.Fatal("invalid response accepted")
	}
	if !r.devices[id].expires.Equal(expiry) {
		t.Fatal("failure renewed renderer")
	}
}

func assertRendererLoadFailure(t *testing.T, failure string) {
	t.Helper()
	actions := []string{}
	r, id := controlledRenderer(t, func(request *http.Request) (*http.Response, error) {
		_, action, _ := strings.Cut(request.Header.Get("SOAPAction"), "#")
		action = strings.Trim(action, `"`)
		actions = append(actions, action)
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if action == "SetAVTransportURI" && !strings.Contains(string(body), "object.item.audioItem.musicTrack") {
			t.Fatal("audio classified as video")
		}
		if action == failure {
			return nil, errors.New("rejected")
		}
		return reply("<ok/>"), nil
	})
	err := r.Load(t.Context(), id, "http://192.168.1.1/media", "audio/mp4", "Song", 20)
	if (err != nil) != (failure != "") {
		t.Fatalf("error=%v", err)
	}
	want := map[string]string{"SetAVTransportURI": "SetAVTransportURI", "Play": "SetAVTransportURI,Play", "Seek": "SetAVTransportURI,Play,Seek,Stop", "": "SetAVTransportURI,Play,Seek"}[failure]
	if strings.Join(actions, ",") != want {
		t.Fatalf("actions=%v want=%s", actions, want)
	}
}
