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

func (adapter *Gateway) ObserveEvents(ctx context.Context, hub *liveevents.Hub) {
	if adapter == nil || adapter.connections == nil || ctx == nil || hub == nil {
		return
	}
	hub.SetObserver(func(profile, name, resource string) { adapter.PublishEvent(ctx, profile, name, resource) })
}

func (adapter *Gateway) PublishEvent(ctx context.Context, profile, name, resource string) {
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

func (adapter *Gateway) deliverEvents(ctx context.Context, id string, delivery *eventDeliveryState) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	defer adapter.endEventDelivery(id, delivery)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, active := adapter.activeEventSubscription(ctx, id); !active {
				return
			}
		case event := <-delivery.queue:
			adapter.deliverQueuedEvent(ctx, id, delivery, event)
		}
	}
}

func (adapter *Gateway) endEventDelivery(id string, delivery *eventDeliveryState) {
	adapter.eventMu.Lock()
	if adapter.eventRuntime[id] == delivery {
		delete(adapter.eventRuntime, id)
	}
	adapter.eventMu.Unlock()
	delivery.cancel()
}

func (adapter *Gateway) activeEventSubscription(ctx context.Context, id string) (eventSubscription, bool) {
	adapter.connections.mu.Lock()
	subscription, found := adapter.connections.events[id]
	adapter.connections.mu.Unlock()
	return subscription, found && adapter.eventAllowed(ctx, subscription)
}

func (adapter *Gateway) deliverQueuedEvent(ctx context.Context, id string, delivery *eventDeliveryState, event eventOccurrence) {
	adapter.eventMu.Lock()
	suspended, generation := delivery.suspended, delivery.generation
	adapter.eventMu.Unlock()
	if suspended {
		return
	}
	failure := adapter.deliverEvent(ctx, id, event)
	if failure == "" {
		return
	}
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

func (adapter *Gateway) deliverEvent(ctx context.Context, id string, event eventOccurrence) string {
	body, _ := json.Marshal(event)
	failure := "connection_refused"
	for attempt := 0; attempt < 3; attempt++ {
		if !waitEventAttempt(ctx, attempt) {
			return "canceled"
		}
		select {
		case adapter.eventSlots <- struct{}{}:
		case <-ctx.Done():
			return "canceled"
		}
		subscription, active := adapter.activeEventSubscription(ctx, id)
		if !active {
			<-adapter.eventSlots
			return "access_unavailable"
		}
		response, err := adapter.sendEventWebhook(ctx, subscription, event.ID, body)
		<-adapter.eventSlots
		var final bool
		if failure, final = eventAttemptOutcome(response, err); final {
			return failure
		}
	}
	return failure
}

func waitEventAttempt(ctx context.Context, attempt int) bool {
	if attempt == 0 {
		return true
	}
	timer := time.NewTimer(time.Duration(1<<attempt) * time.Second)
	select {
	case <-ctx.Done():
		timer.Stop()
		return false
	case <-timer.C:
		return true
	}
}

func eventAttemptOutcome(response *http.Response, err error) (string, bool) {
	if err != nil {
		return eventCallbackError(err), false
	}
	_ = response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return "", true
	}
	return eventResponseError(response.StatusCode), response.StatusCode == http.StatusGone || response.StatusCode == http.StatusRequestEntityTooLarge
}
