package mcpgateway

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (adapter *Gateway) subscribeEvent(ctx context.Context, _ *mcp.ServerSession, params *eventParams) (*eventResult, error) { //nolint:cyclop,gocognit // Verification and atomic persistence are ordered as one subscription transaction.
	subscription, err := adapter.prepareEventSubscription(ctx, params)
	if err != nil {
		return nil, err
	}
	adapter.eventSubscribe.Lock()
	defer adapter.eventSubscribe.Unlock()
	connections := adapter.connections
	connections.mu.Lock()
	state := connections.stateLocked()
	pruneExpiredEvents(state.Events, connections.now().Unix())
	previous, exists := state.Events[subscription.ID]
	connections.mu.Unlock()
	if connections.err != nil {
		return nil, eventError(-32013, "event persistence is unavailable", nil)
	}
	if !exists && len(state.Events) >= eventSubscriptionLimit {
		return nil, eventError(-32013, "event subscription limit reached", map[string]any{"limit": "subscriptions", "max": eventSubscriptionLimit})
	}
	if err := adapter.verifyEventCallback(ctx, subscription); err != nil {
		return nil, err
	}
	if !adapter.eventAllowed(ctx, subscription) {
		return nil, eventError(-32012, "event access is unavailable", nil)
	}
	if exists && previous.Secret != subscription.Secret {
		subscription.OldSecret, subscription.RotateUntil = previous.Secret, connections.now().Add(5*time.Minute).Unix()
	} else if exists {
		subscription.OldSecret, subscription.RotateUntil = previous.OldSecret, previous.RotateUntil
	}
	connections.mu.Lock()
	state = connections.stateLocked()
	pruneExpiredEvents(state.Events, connections.now().Unix())
	state.Events[subscription.ID] = subscription
	err = connections.commitLocked(state)
	connections.mu.Unlock()
	if err != nil {
		return nil, eventError(-32013, "event persistence is unavailable", nil)
	}
	adapter.eventMu.Lock()
	truncated := false
	if delivery := adapter.eventRuntime[subscription.ID]; delivery != nil {
		truncated = delivery.truncated
		delivery.generation++
		delivery.truncated = false
		delivery.suspended = false
	}
	adapter.eventMu.Unlock()
	return &eventResult{ID: subscription.ID, RefreshBefore: time.Unix(subscription.Expires, 0).UTC().Format(time.RFC3339), Truncated: truncated}, nil
}

func (adapter *Gateway) prepareEventSubscription(ctx context.Context, params *eventParams) (eventSubscription, error) {
	if err := validateEventParams(params, false); err != nil {
		return eventSubscription{}, err
	}
	subscription, err := adapter.eventIdentity(ctx, params.Name)
	if err != nil {
		return eventSubscription{}, err
	}
	subscription.URL, subscription.Resource, subscription.Secret = params.Delivery.URL, params.Arguments["resource"], params.Delivery.Secret
	subscription.ID = eventID(subscription)
	ttl, _ := eventTTL(params.TTL)
	subscription.Expires = adapter.connections.now().Add(ttl).Unix()
	if subscription.AuthorityExpires != 0 {
		subscription.Expires = min(subscription.Expires, subscription.AuthorityExpires)
	}
	if !adapter.eventAllowed(ctx, subscription) {
		return eventSubscription{}, eventError(-32012, "event access is unavailable", nil)
	}
	return subscription, nil
}

func (adapter *Gateway) unsubscribeEvent(ctx context.Context, _ *mcp.ServerSession, params *eventParams) (*mcp.ResultBase, error) {
	if err := validateEventParams(params, true); err != nil {
		return nil, err
	}
	subscription, err := adapter.eventIdentity(ctx, params.Name)
	if err != nil {
		return nil, err
	}
	subscription.URL, subscription.Resource = params.Delivery.URL, params.Arguments["resource"]
	id := eventID(subscription)
	adapter.eventSubscribe.Lock()
	defer adapter.eventSubscribe.Unlock()
	connections := adapter.connections
	connections.mu.Lock()
	state := connections.stateLocked()
	if _, exists := state.Events[id]; exists {
		delete(state.Events, id)
		err = connections.commitLocked(state)
	}
	connections.mu.Unlock()
	if err != nil {
		return nil, eventError(-32013, "event persistence is unavailable", nil)
	}
	adapter.stopEventDelivery(id)
	return &mcp.ResultBase{}, nil
}

func (adapter *Gateway) stopEventDelivery(id string) {
	adapter.eventMu.Lock()
	defer adapter.eventMu.Unlock()
	if delivery := adapter.eventRuntime[id]; delivery != nil {
		delivery.cancel()
		delete(adapter.eventRuntime, id)
	}
}

func pruneExpiredEvents(subscriptions map[string]eventSubscription, now int64) {
	for id, subscription := range subscriptions {
		if subscription.Expires <= now {
			delete(subscriptions, id)
		}
	}
}
