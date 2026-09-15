package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type subtitleShellWriter struct {
	*httptest.ResponseRecorder
	onFlush func()
}

func (writer *subtitleShellWriter) Flush() {
	writer.ResponseRecorder.Flush()
	writer.onFlush()
}

func TestSubtitleDashboardFlushesNavigationBeforeReadingLibrary(t *testing.T) {
	t.Parallel()
	for _, view := range []string{"summary", "wanted", "library"} {
		t.Run(view, func(t *testing.T) {
			manager, _ := subtitleFactsFixture(t, "#!/bin/sh\nexit 1\n")
			index := manager.index
			manager.index = nil // Any projection before Flush would panic.
			response := httptest.NewRecorder()
			writer := &subtitleShellWriter{ResponseRecorder: response, onFlush: func() {
				body := response.Body.String()
				for _, expected := range []string{`class="app-header"`, `href="/settings"`, `id="main"`} {
					if !strings.Contains(body, expected) {
						t.Errorf("initial response missing %q", expected)
					}
				}
				if strings.Contains(body, "subtitle-browser") || strings.Contains(body, "</html>") {
					t.Error("coverage was rendered before the initial flush")
				}
				manager.index = index
			}}
			manager.dashboard(writer, ownerRequest("/?view="+view+"&q=%3CFilm%3E"))
			if !response.Flushed || response.Code != http.StatusOK || response.Header().Get("X-Accel-Buffering") != "no" {
				t.Fatalf("shell was not streamed: %#v", response)
			}
			if !strings.Contains(response.Body.String(), "&lt;Film&gt;") || !strings.Contains(response.Body.String(), "subtitle-browser") || !strings.HasSuffix(response.Body.String(), "</main></body></html>") {
				t.Fatal("stream did not finish with the dashboard content")
			}
		})
	}
}

func TestSubtitleDashboardInvalidQueryDoesNotFlushOrReadLibrary(t *testing.T) {
	t.Parallel()
	manager := &subtitleManager{}
	for _, path := range []string{"/?view=invalid", "/?view=wanted&view=library", "/?unknown=true", "/?q=" + strings.Repeat("x", 129)} {
		response := httptest.NewRecorder()
		manager.dashboard(response, ownerRequest(path))
		if response.Code != http.StatusBadRequest || response.Flushed {
			t.Fatalf("invalid request streamed a dashboard: %d, flushed=%v", response.Code, response.Flushed)
		}
	}
}

func TestSubtitleDashboardDisconnectAfterShellSkipsProjection(t *testing.T) {
	t.Parallel()
	manager, _ := subtitleFactsFixture(t, "#!/bin/sh\nexit 1\n")
	manager.index = nil
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	response := httptest.NewRecorder()
	writer := &subtitleShellWriter{ResponseRecorder: response, onFlush: cancel}
	manager.dashboard(writer, ownerRequest("/").WithContext(ctx))
	if !response.Flushed || strings.Contains(response.Body.String(), "subtitle-browser") {
		t.Fatal("disconnected request continued projecting the library")
	}
}
