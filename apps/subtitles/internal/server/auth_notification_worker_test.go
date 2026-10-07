package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// New's delivery worker reads Done synchronously before launching. This owned
// context observes registration without goroutine-count races or waiting sleeps.
type notificationLifecycleProbe struct {
	context.Context
	registrations atomic.Int32
}

func (probe *notificationLifecycleProbe) Done() <-chan struct{} {
	probe.registrations.Add(1)
	return probe.Context.Done()
}

// Failure modes: an absent/invalid notifier still starts a delivery worker;
// omitting the callback accidentally disables durable audit; valid notifier
// delivery or supported enabling after restart is lost. Public webhook delivery
// remains covered by TestWebhookReceivesRedactedAdministrativeEvents in each app.
func TestUnavailableNotifierDoesNotRegisterWorkerAndKeepsAudit(t *testing.T) {
	for _, value := range []struct {
		name, address, status string
	}{
		{"absent", "", "Not configured"},
		{"invalid scheme", "ftp://127.0.0.1/event", "Configuration error"},
		{"credentials", "http://user:pass@127.0.0.1/event", "Configuration error"},
		{"fragment", "http://127.0.0.1/event#private", "Configuration error"},
	} {
		t.Run(value.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			probe := &notificationLifecycleProbe{Context: ctx}
			data := t.TempDir()
			auth := newAuthentication(probe, data, true, "", newSettingsStore("", data, "", nil), NotificationConfig{URL: value.address}, nil)
			if auth.notify.status != value.status {
				t.Fatalf("notifier status = %q", auth.notify.status)
			}
			if probe.registrations.Load() != 0 {
				t.Fatal("unavailable notifier registered a delivery worker")
			}
			auth.audit.Denied(httptest.NewRequestWithContext(ctx, "GET", "/settings", nil), "fixture denial")
			events := auth.audit.Query("security", 10)
			if len(events) != 1 || events[0].Action != "access.denied" || !auth.audit.Healthy() {
				t.Fatal("unavailable notifier lost durable audit")
			}
			reloaded := newAuditStore(ctx, data, nil)
			if events := reloaded.Query("security", 10); len(events) != 1 || events[0].Action != "access.denied" {
				t.Fatal("durable audit did not survive reload")
			}
		})
	}
}

func TestRestartedNotifierRegistersValidDeliveryOwner(t *testing.T) {
	data := t.TempDir()
	disabledContext, cancelDisabled := context.WithCancel(t.Context())
	defer cancelDisabled()
	disabled := &notificationLifecycleProbe{Context: disabledContext}
	first := newAuthentication(disabled, data, true, "", newSettingsStore("", data, "", nil), NotificationConfig{}, nil)
	if first.notify.status != "Not configured" || disabled.registrations.Load() != 0 {
		t.Fatal("disabled process retained an unnecessary worker")
	}
	// Deployment webhook fields require restart. Reconstruct authentication from
	// the same durable state and the next startup's supported validated config.
	delivered := make(chan string, 1)
	webhook := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		delivered <- request.Header.Get("Authorization")
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer webhook.Close()
	enabledContext, cancelEnabled := context.WithCancel(t.Context())
	defer cancelEnabled()
	enabled := &notificationLifecycleProbe{Context: enabledContext}
	next := newAuthentication(enabled, data, true, "", newSettingsStore("", data, "", nil), NotificationConfig{URL: webhook.URL, Token: "fixture-token"}, nil)
	if next.notify.status != "Enabled" || enabled.registrations.Load() != 1 {
		t.Fatal("valid restarted notifier lost its lifecycle owner")
	}
	next.audit.Denied(httptest.NewRequestWithContext(enabledContext, "GET", "/settings", nil), "restart fixture denial")
	select {
	case authorization := <-delivered:
		if authorization != "Bearer fixture-token" {
			t.Fatal("restarted notifier lost its configured credential")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("restarted notifier did not deliver its audit event")
	}
}
