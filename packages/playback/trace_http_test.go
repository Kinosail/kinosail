package playback

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTraceHTTPValidatesAndRecordsPlayerEvent(t *testing.T) {
	request := traceRequest(t, `{"session":"session","event":"playing","sequence":1}`)
	response := httptest.NewRecorder()
	setSession := ""
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	TraceHTTP(TraceHTTPConfig{
		Visible:      func(got *http.Request, id string) bool { return got == request && id == "film" },
		ValidSession: func(session string) bool { return session == "session" },
		SetSession: func(got *http.Request, session string) {
			if got != request {
				t.Fatal("session request was replaced")
			}
			setSession = session
		},
	})(response, request)
	if response.Code != http.StatusNoContent || setSession != "session" || !strings.Contains(logs.String(), `"level":"INFO"`) || !strings.Contains(logs.String(), `"msg":"playback trace"`) || !strings.Contains(logs.String(), `"event":"playing"`) {
		t.Fatalf("response = %d; session = %q; log = %s", response.Code, setSession, logs.String())
	}
}

func TestTraceHTTPRecordsFullscreenFailureAtWarningLevel(t *testing.T) {
	for _, name := range []string{"NotAllowedError", "InvalidStateError", "NotSupportedError", "TypeError", "Error"} {
		t.Run(name, func(t *testing.T) {
			var logs bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })
			response := httptest.NewRecorder()
			TraceHTTP(TraceHTTPConfig{
				Visible:      func(*http.Request, string) bool { return true },
				ValidSession: func(value string) bool { return value == "session" },
				SetSession:   func(*http.Request, string) {},
			})(response, traceRequest(t, `{"session":"session","event":"error","sequence":1,"detail":"fullscreen:`+name+`:playback-retained"}`))
			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d", response.Code)
			}
			for _, field := range []string{`"level":"WARN"`, `"playback_session":"session"`, `"detail":"fullscreen:` + name + `:playback-retained"`} {
				if !strings.Contains(logs.String(), field) {
					t.Fatalf("missing %s in %s", field, logs.String())
				}
			}
		})
	}
}

func TestTraceHTTPRejectsBeforeSessionSideEffects(t *testing.T) {
	for _, test := range []struct {
		name, body string
		visible    bool
		status     int
	}{
		{"hidden", `{"session":"session","event":"playing","sequence":1}`, false, http.StatusNotFound},
		{"malformed", `{`, true, http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			set := false
			response := httptest.NewRecorder()
			TraceHTTP(TraceHTTPConfig{
				Visible:      func(*http.Request, string) bool { return test.visible },
				ValidSession: func(string) bool { return true },
				SetSession:   func(*http.Request, string) { set = true },
			})(response, traceRequest(t, test.body))
			if response.Code != test.status || set {
				t.Fatalf("response = %d %q; set = %t", response.Code, response.Body.String(), set)
			}
		})
	}
}

func TestTraceHTTPFailsClosedWithoutBindings(t *testing.T) {
	response := httptest.NewRecorder()
	TraceHTTP(TraceHTTPConfig{})(response, traceRequest(t, `{"session":"session","event":"playing","sequence":1}`))
	if response.Code != http.StatusInternalServerError || response.Body.String() != `{"error":"playback trace unavailable"}`+"\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func traceRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/items/film/playback-events", strings.NewReader(body))
	request.SetPathValue("id", "film")
	return request
}

func TestTraceAcceptsMovingFrameEvidence(t *testing.T) {
	request := traceRequest(t, `{"session":"session","event":"first-moving-frame","sequence":3,"elapsedMs":400,"totalFrames":2}`)
	event, err := ReadTrace(httptest.NewRecorder(), request, func(value string) bool { return value == "session" })
	if err != nil || event.Event != "first-moving-frame" || event.TotalFrames != 2 {
		t.Fatalf("moving frame evidence = %#v, %v", event, err)
	}
}

func TestTraceHTTPRejectsLogInjectionWithoutEffects(t *testing.T) {
	for _, field := range []string{"session", "event", "method", "detail", "quality", "visibility"} {
		for _, attack := range []string{"valid\r\n{\"level\":\"ERROR\"}", "\x1b[2J", "<script>alert(1)</script>", strings.Repeat("x", 4097)} {
			t.Run(field+"/"+attack[:1], func(t *testing.T) {
				body := map[string]any{"session": "session", "event": "playing", "sequence": 1}
				body[field] = attack
				encoded, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				var logs bytes.Buffer
				previous := slog.Default()
				slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
				t.Cleanup(func() { slog.SetDefault(previous) })
				set := false
				response := httptest.NewRecorder()
				TraceHTTP(TraceHTTPConfig{
					Visible:      func(*http.Request, string) bool { return true },
					ValidSession: func(value string) bool { return value == "session" },
					SetSession:   func(*http.Request, string) { set = true },
				})(response, traceRequest(t, string(encoded)))
				if response.Code != http.StatusBadRequest || set || logs.Len() != 0 {
					t.Fatalf("status=%d set=%t logs=%q", response.Code, set, logs.String())
				}
			})
		}
	}
}

func TestTraceHTTPRetainsOnlyNamedPlayRejections(t *testing.T) {
	for _, fixture := range []struct{ event, detail, expected string }{
		{"play-rejected", "control:NotAllowedError", "control:NotAllowedError"},
		{"play-rejected", "autoplay-canplay:NotSupportedError", "autoplay-canplay:NotSupportedError"},
		{"play-rejected", "keyboard:AbortError", "keyboard:AbortError"},
		{"play-rejected", "media-element:InvalidStateError", "media-element:InvalidStateError"},
		{"play-rejected", "watch-room:TypeError", "watch-room:TypeError"},
		{"play-rejected", "media-session:Error", "media-session:Error"},
		{"play-rejected", "queue-advance:NotAllowedError", "queue-advance:NotAllowedError"},
		{"play-rejected", "resume-progress:AbortError", "resume-progress:AbortError"},
		{"play-rejected", "home-assistant:NotSupportedError", "home-assistant:NotSupportedError"},
		{"play-rejected", "source-change:AbortError", "source-change:AbortError"},
		{"play-rejected", "offline-source:NotAllowedError", "offline-source:NotAllowedError"},
		{"play-rejected", "qa_sensitive_marker:NotAllowedError", ""},
		{"play-rejected", "control:qa_sensitive_marker", ""},
		{"play-rejected", "control:Error:qa_sensitive_marker", ""},
		{"play-rejected", ":NotAllowedError", ""},
		{"playing", "control:NotAllowedError", ""},
		{"error", "control:NotAllowedError", ""},
	} {
		t.Run(fixture.event+"/"+fixture.detail, func(t *testing.T) {
			var logs bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })
			body, err := json.Marshal(map[string]any{"session": "session", "event": fixture.event, "sequence": 1, "detail": fixture.detail})
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			setSession := false
			TraceHTTP(TraceHTTPConfig{
				Visible:      func(*http.Request, string) bool { return true },
				ValidSession: func(value string) bool { return value == "session" },
				SetSession:   func(*http.Request, string) { setSession = true },
			})(response, traceRequest(t, string(body)))
			if response.Code != http.StatusNoContent || !setSession {
				t.Fatalf("status=%d sessionSet=%t", response.Code, setSession)
			}
			var entry struct {
				Detail string `json:"detail"`
			}
			if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if entry.Detail != fixture.expected || strings.Contains(logs.String(), "qa_sensitive_marker") {
				t.Fatalf("logged detail = %q, want %q", entry.Detail, fixture.expected)
			}
		})
	}
}
