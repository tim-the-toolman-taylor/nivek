package overlayrelay

import (
	"testing"
)

func TestRegistryPushDeliversToAttachedOverlay(t *testing.T) {
	r := NewRegistry()
	conn := r.Add(7, func() {})

	r.Push(7, ServerFrame{Type: MsgEvent, Event: &Event{Seq: 1}})

	select {
	case frame := <-conn.Out():
		if frame.Event == nil || frame.Event.Seq != 1 {
			t.Fatalf("unexpected frame: %+v", frame)
		}
	default:
		t.Fatal("frame was not queued")
	}
}

func TestRegistryPushToAbsentOverlayIsNotAnError(t *testing.T) {
	r := NewRegistry()
	// The event is already durable in the log; an offline overlay replays it
	// from its cursor. This must not panic or block.
	r.Push(1234, ServerFrame{Type: MsgEvent, Event: &Event{Seq: 1}})

	if r.IsConnected(1234) {
		t.Fatal("IsConnected true for a user that never attached")
	}
}

func TestRegistryPushDropsWedgedClientRatherThanBlocking(t *testing.T) {
	r := NewRegistry()

	cancelled := false
	conn := r.Add(7, func() { cancelled = true })

	// Fill the outbox, then overflow it. The overflow must not block the
	// caller: Push runs on the EventSub ingest path.
	for i := 0; i < outboxDepth+5; i++ {
		r.Push(7, ServerFrame{Type: MsgEvent, Event: &Event{Seq: int64(i)}})
	}

	if !cancelled {
		t.Fatal("overflowing client was not cancelled")
	}
	if len(conn.Out()) != outboxDepth {
		t.Fatalf("outbox depth = %d, want %d", len(conn.Out()), outboxDepth)
	}
}

func TestRegistryAddDisplacesStaleConnection(t *testing.T) {
	r := NewRegistry()

	firstCancelled := false
	first := r.Add(7, func() { firstCancelled = true })
	second := r.Add(7, func() {})

	if !firstCancelled {
		t.Fatal("reconnect did not tear down the stale connection")
	}
	if !r.IsConnected(7) {
		t.Fatal("user should still be connected via the replacement")
	}

	// The displaced connection must not evict its replacement on teardown.
	r.Remove(first)
	if !r.IsConnected(7) {
		t.Fatal("stale connection evicted its replacement")
	}

	r.Remove(second)
	if r.IsConnected(7) {
		t.Fatal("registry still reports connected after the live conn left")
	}
}

func TestRegistryCount(t *testing.T) {
	r := NewRegistry()
	if r.Count() != 0 {
		t.Fatalf("empty registry Count = %d", r.Count())
	}

	a := r.Add(1, func() {})
	r.Add(2, func() {})
	if r.Count() != 2 {
		t.Fatalf("Count = %d, want 2", r.Count())
	}

	r.Remove(a)
	if r.Count() != 1 {
		t.Fatalf("Count after remove = %d, want 1", r.Count())
	}
}

func TestConnCloseIsIdempotent(t *testing.T) {
	calls := 0
	r := NewRegistry()
	conn := r.Add(7, func() { calls++ })

	conn.Close()
	conn.Close()
	r.Remove(conn)

	if calls != 1 {
		t.Fatalf("cancel called %d times, want 1", calls)
	}
}
