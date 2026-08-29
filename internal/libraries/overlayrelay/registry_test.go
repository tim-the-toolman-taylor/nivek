package overlayrelay

import (
	"testing"
)

func TestRegistryPushDeliversToAttachedOverlay(t *testing.T) {
	r := NewRegistry()
	conn := r.Add(7, 1, func() {})

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
	conn := r.Add(7, 1, func() { cancelled = true })

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
	first := r.Add(7, 1, func() { firstCancelled = true })
	second := r.Add(7, 2, func() {})

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

	a := r.Add(1, 1, func() {})
	r.Add(2, 1, func() {})
	if r.Count() != 2 {
		t.Fatalf("Count = %d, want 2", r.Count())
	}

	r.Remove(a)
	if r.Count() != 1 {
		t.Fatalf("Count after remove = %d, want 1", r.Count())
	}
}

func TestRegistryConnectedDeviceID(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.ConnectedDeviceID(7); ok {
		t.Fatal("reported a connected device for an unattached user")
	}

	r.Add(7, 42, func() {})
	got, ok := r.ConnectedDeviceID(7)
	if !ok || got != 42 {
		t.Fatalf("ConnectedDeviceID = (%d, %v), want (42, true)", got, ok)
	}
}

func TestRegistryDisconnectDevice(t *testing.T) {
	r := NewRegistry()
	cancelled := false
	r.Add(7, 42, func() { cancelled = true })

	// Revoking a different device of the same user leaves the live socket alone.
	r.DisconnectDevice(7, 99)
	if cancelled || !r.IsConnected(7) {
		t.Fatal("disconnecting a different device tore down the live connection")
	}

	// Revoking the attached device tears its socket down.
	r.DisconnectDevice(7, 42)
	if !cancelled {
		t.Fatal("revoking the attached device did not disconnect it")
	}
}

func TestConnCloseIsIdempotent(t *testing.T) {
	calls := 0
	r := NewRegistry()
	conn := r.Add(7, 1, func() { calls++ })

	conn.Close()
	conn.Close()
	r.Remove(conn)

	if calls != 1 {
		t.Fatalf("cancel called %d times, want 1", calls)
	}
}
