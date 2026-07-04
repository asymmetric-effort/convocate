package events

import (
	"testing"
	"time"
)

func newTestHub() *Hub {
	return &Hub{
		subscribers: make(map[string]map[*subscriber]struct{}),
	}
}

func TestHubSubscribe(t *testing.T) {
	h := newTestHub()
	sub := h.Subscribe("ch1", nil)
	if sub == nil {
		t.Fatal("expected non-nil subscriber")
	}
	if sub.ch == nil {
		t.Fatal("expected non-nil channel")
	}
	if sub.done == nil {
		t.Fatal("expected non-nil done channel")
	}
	if sub.types != nil {
		t.Fatal("expected nil type filter when no filter given")
	}
}

func TestHubSubscribeWithTypeFilter(t *testing.T) {
	h := newTestHub()
	sub := h.Subscribe("ch1", []string{"node.updated", "node.deleted"})
	if sub.types == nil {
		t.Fatal("expected non-nil type filter")
	}
	if !sub.types["node.updated"] {
		t.Fatal("expected node.updated in filter")
	}
	if !sub.types["node.deleted"] {
		t.Fatal("expected node.deleted in filter")
	}
	if sub.types["node.created"] {
		t.Fatal("unexpected node.created in filter")
	}
	h.Unsubscribe("ch1", sub)
}

func TestHubPublishAndReceive(t *testing.T) {
	h := newTestHub()
	sub := h.Subscribe("ch1", nil)
	defer h.Unsubscribe("ch1", sub)

	h.Publish("ch1", "test.event", map[string]string{"key": "value"})

	select {
	case data := <-sub.ch:
		if len(data) == 0 {
			t.Fatal("expected non-empty data")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestHubPublishFilteredEvent(t *testing.T) {
	h := newTestHub()
	sub := h.Subscribe("ch1", []string{"wanted"})
	defer h.Unsubscribe("ch1", sub)

	// Publish an event type NOT in the filter
	h.Publish("ch1", "unwanted", "payload")

	select {
	case <-sub.ch:
		t.Fatal("should not receive filtered-out event")
	case <-time.After(50 * time.Millisecond):
		// Expected: no event received
	}

	// Publish an event type IN the filter
	h.Publish("ch1", "wanted", "payload")

	select {
	case data := <-sub.ch:
		if len(data) == 0 {
			t.Fatal("expected non-empty data")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for wanted event")
	}
}

func TestHubMultipleSubscribers(t *testing.T) {
	h := newTestHub()
	sub1 := h.Subscribe("ch1", nil)
	sub2 := h.Subscribe("ch1", nil)
	defer h.Unsubscribe("ch1", sub1)
	defer h.Unsubscribe("ch1", sub2)

	h.Publish("ch1", "broadcast", "data")

	for i, sub := range []*subscriber{sub1, sub2} {
		select {
		case data := <-sub.ch:
			if len(data) == 0 {
				t.Fatalf("subscriber %d: expected non-empty data", i)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out", i)
		}
	}
}

func TestHubUnsubscribe(t *testing.T) {
	h := newTestHub()
	sub := h.Subscribe("ch1", nil)

	h.Unsubscribe("ch1", sub)

	// Verify channel map is cleaned up
	h.mu.RLock()
	if _, ok := h.subscribers["ch1"]; ok {
		t.Fatal("expected channel to be removed after last subscriber unsubscribes")
	}
	h.mu.RUnlock()

	// Verify done channel is closed
	select {
	case <-sub.done:
		// Expected
	default:
		t.Fatal("expected done channel to be closed")
	}
}

func TestHubUnsubscribeNonexistentChannel(t *testing.T) {
	h := newTestHub()
	sub := &subscriber{
		ch:   make(chan []byte, 64),
		done: make(chan struct{}),
	}
	// Should not panic
	h.Unsubscribe("nonexistent", sub)
}

func TestHubPublishToNonexistentChannel(t *testing.T) {
	h := newTestHub()
	// Should not panic
	h.Publish("nonexistent", "test", "data")
}

func TestHubPublishDropsSlowSubscriber(t *testing.T) {
	h := newTestHub()
	sub := h.Subscribe("ch1", nil)
	defer h.Unsubscribe("ch1", sub)

	// Fill the subscriber's buffer (capacity 64)
	for i := 0; i < 65; i++ {
		h.Publish("ch1", "test", i)
	}

	// Drain what we can
	count := 0
	for {
		select {
		case <-sub.ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != 64 {
		t.Fatalf("expected 64 messages (buffer capacity), got %d", count)
	}
}

func TestHubChannelIsolation(t *testing.T) {
	h := newTestHub()
	sub1 := h.Subscribe("ch1", nil)
	sub2 := h.Subscribe("ch2", nil)
	defer h.Unsubscribe("ch1", sub1)
	defer h.Unsubscribe("ch2", sub2)

	h.Publish("ch1", "test", "data")

	select {
	case <-sub2.ch:
		t.Fatal("subscriber on ch2 should not receive ch1 events")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}
}

func TestHubMultipleUnsubscribePartial(t *testing.T) {
	h := newTestHub()
	sub1 := h.Subscribe("ch1", nil)
	sub2 := h.Subscribe("ch1", nil)

	h.Unsubscribe("ch1", sub1)

	// ch1 should still exist with sub2
	h.mu.RLock()
	subs, ok := h.subscribers["ch1"]
	h.mu.RUnlock()
	if !ok {
		t.Fatal("expected channel to still exist with remaining subscriber")
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscriber, got %d", len(subs))
	}

	h.Unsubscribe("ch1", sub2)
}

func TestHubPublishMarshalError(t *testing.T) {
	h := newTestHub()
	sub := h.Subscribe("ch1", nil)
	defer h.Unsubscribe("ch1", sub)

	// Publish with a payload that can't be marshaled
	h.Publish("ch1", "test", make(chan int))

	// Should not receive any event since marshal fails
	select {
	case <-sub.ch:
		t.Fatal("should not receive event when marshal fails")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}
}

func TestDefaultHub(t *testing.T) {
	if DefaultHub == nil {
		t.Fatal("DefaultHub should be initialized")
	}
	if DefaultHub.subscribers == nil {
		t.Fatal("DefaultHub.subscribers should be initialized")
	}
}
