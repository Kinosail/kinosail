package mcpgateway

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/liveevents"
)

type eventGateway struct {
	connections             *Connections
	subtitleEvents          bool
	eventMu, eventSubscribe sync.Mutex
	eventVerify             httpguard.Limiter
	eventVerified           map[string]int64
	eventRuntime            map[string]*eventDeliveryState
	eventSlots              chan struct{}
}

func newEventGateway(config GatewayConfig) eventGateway {
	return eventGateway{subtitleEvents: config.SubtitleEvents, eventRuntime: make(map[string]*eventDeliveryState), eventVerified: make(map[string]int64), eventSlots: make(chan struct{}, 8)}
}

type eventOccurrence struct {
	ID        string            `json:"eventId"`
	Name      string            `json:"name"`
	Timestamp string            `json:"timestamp"`
	Data      map[string]string `json:"data"`
	Cursor    *string           `json:"cursor"`
}

type eventDeliveryState struct {
	queue      chan eventOccurrence
	cancel     context.CancelFunc
	truncated  bool
	suspended  bool
	generation uint64
}

// ObserveEvents connects the application's live updates to its MCP subscriptions.
func (adapter *Gateway) ObserveEvents(ctx context.Context, hub *liveevents.Hub) {
	if adapter == nil || adapter.connections == nil || ctx == nil || hub == nil {
		return
	}
	hub.SetObserver(func(profile, name, resource string) { adapter.PublishEvent(ctx, profile, name, resource) })
}

// PublishEvent queues a minimal notice within its optional Viewer Profile scope.
func (adapter *Gateway) PublishEvent(ctx context.Context, profile, name, resource string) { //nolint:cyclop,gocognit // Profile filtering and bounded queue admission stay in one publication boundary.
	if adapter == nil || adapter.connections == nil || ctx == nil || !eventResource(name, resource) || resource == "" {
		return
	}
	event := eventOccurrence{ID: "evt_" + rand.Text(), Name: name, Timestamp: adapter.connections.now().UTC().Format(time.RFC3339Nano), Data: map[string]string{"resource": resource}}
	adapter.connections.mu.Lock()
	subscriptions := make([]eventSubscription, 0, len(adapter.connections.events))
	for _, subscription := range adapter.connections.events {
		if subscription.Name == name && (profile == "" || profile == subscription.Principal.ID) && (subscription.Resource == "" || subscription.Resource == resource) {
			subscriptions = append(subscriptions, subscription)
		}
	}
	adapter.connections.mu.Unlock()
	for _, subscription := range subscriptions {
		if !adapter.eventAllowed(ctx, subscription) {
			adapter.stopEventDelivery(subscription.ID)
			continue
		}
		adapter.eventMu.Lock()
		delivery := adapter.eventRuntime[subscription.ID]
		if delivery == nil {
			lifecycle, cancel := context.WithCancel(ctx)
			delivery = &eventDeliveryState{queue: make(chan eventOccurrence, 16), cancel: cancel}
			adapter.eventRuntime[subscription.ID] = delivery
			go adapter.deliverEvents(lifecycle, subscription.ID, delivery)
		}
		if delivery.suspended {
			delivery.truncated = true
		} else {
			select {
			case delivery.queue <- event:
			default:
				delivery.truncated = true
			}
		}
		adapter.eventMu.Unlock()
	}
}

func (adapter *Gateway) deliverEvents(ctx context.Context, id string, delivery *eventDeliveryState) { //nolint:cyclop,gocognit // Queue state, refresh generations, and teardown stay in one worker loop.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	defer func() {
		adapter.eventMu.Lock()
		if adapter.eventRuntime[id] == delivery {
			delete(adapter.eventRuntime, id)
		}
		adapter.eventMu.Unlock()
		delivery.cancel()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			adapter.connections.mu.Lock()
			subscription, found := adapter.connections.events[id]
			adapter.connections.mu.Unlock()
			if !found || !adapter.eventAllowed(ctx, subscription) {
				return
			}
		case event := <-delivery.queue:
			adapter.eventMu.Lock()
			suspended, generation := delivery.suspended, delivery.generation
			adapter.eventMu.Unlock()
			if suspended {
				continue
			}
			if failure := adapter.deliverEvent(ctx, id, event); failure != "" {
				adapter.eventMu.Lock()
				delivery.truncated = true
				if delivery.generation == generation {
					delivery.suspended = true
				}
				adapter.eventMu.Unlock()
				if ctx.Err() == nil {
					slog.Warn("MCP event delivery failed", "operation", "events/deliver", "subscription_id", id, "event_id", event.ID, "failure", failure)
				}
			}
		}
	}
}

func (adapter *Gateway) deliverEvent(ctx context.Context, id string, event eventOccurrence) string { //nolint:cyclop,gocognit // Backoff, authority rechecks, and concurrency slots form one bounded delivery lifecycle.
	body, _ := json.Marshal(event)
	failure := "connection_refused"
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(time.Duration(1<<attempt) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "canceled"
			case <-timer.C:
			}
		}
		select {
		case adapter.eventSlots <- struct{}{}:
		case <-ctx.Done():
			return "canceled"
		}
		adapter.connections.mu.Lock()
		subscription, found := adapter.connections.events[id]
		adapter.connections.mu.Unlock()
		if !found || !adapter.eventAllowed(ctx, subscription) {
			<-adapter.eventSlots
			return "access_unavailable"
		}
		response, err := adapter.sendEventWebhook(ctx, subscription, event.ID, body)
		<-adapter.eventSlots
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				return ""
			}
			failure = eventResponseError(response.StatusCode)
			if response.StatusCode == http.StatusGone || response.StatusCode == http.StatusRequestEntityTooLarge {
				return failure
			}
		} else {
			failure = eventCallbackError(err)
		}
	}
	return failure
}
