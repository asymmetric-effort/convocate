package events

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/asymmetric-effort/convocate/internal/types"
)

type subscriber struct {
	ch     chan []byte
	done   chan struct{}
	// types restricts which event types this subscriber receives.
	// nil means all events pass (backward compatible).
	types  map[string]bool
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[*subscriber]struct{} // channel -> subscribers
}

var DefaultHub = &Hub{
	subscribers: make(map[string]map[*subscriber]struct{}),
}

// Subscribe registers a new subscriber for the given channel.
// If typeFilter is non-empty, only events whose Type matches one
// of the filter values will be delivered.
func (h *Hub) Subscribe(channel string, typeFilter []string) *subscriber {
	h.mu.Lock()
	defer h.mu.Unlock()

	sub := &subscriber{
		ch:   make(chan []byte, 64),
		done: make(chan struct{}),
	}

	if len(typeFilter) > 0 {
		sub.types = make(map[string]bool, len(typeFilter))
		for _, t := range typeFilter {
			sub.types[t] = true
		}
	}

	if h.subscribers[channel] == nil {
		h.subscribers[channel] = make(map[*subscriber]struct{})
	}
	h.subscribers[channel][sub] = struct{}{}
	return sub
}

func (h *Hub) Unsubscribe(channel string, sub *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subs, ok := h.subscribers[channel]; ok {
		delete(subs, sub)
		if len(subs) == 0 {
			delete(h.subscribers, channel)
		}
	}
	close(sub.done)
}

func (h *Hub) Publish(channel string, eventType string, payload any) {
	evt := types.Event{
		Type:      eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   payload,
	}
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	subs := h.subscribers[channel]
	for sub := range subs {
		// Skip if subscriber has a type filter and this event doesn't match
		if sub.types != nil && !sub.types[eventType] {
			continue
		}
		select {
		case sub.ch <- data:
		default:
			// Drop message if subscriber is slow
		}
	}
}
