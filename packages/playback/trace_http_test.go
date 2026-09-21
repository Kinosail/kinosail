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
	if response.Code != http.StatusNoContent || setSession != "session" || !strings.Contains(logs.String(), `"msg":"playback trace"`) || !strings.Contains(logs.String(), `"event":"playing"`) {
		t.Fatalf("response = %d; session = %q; log = %s", response.Code, setSession, logs.String())
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
