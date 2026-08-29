package overlayrelay

import (
	"sync"
	"testing"
)

func TestKeyedMutexSerializesSameKey(t *testing.T) {
	km := NewKeyedMutex()

	// Many goroutines contend on one key. If Lock did not serialise, the
	// unguarded counter would race (caught by -race) and likely miss increments.
	const goroutines, perG = 50, 100
	counter := 0
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				unlock := km.Lock("chan-1")
				counter++
				unlock()
			}
		}()
	}
	wg.Wait()

	if counter != goroutines*perG {
		t.Fatalf("counter = %d, want %d", counter, goroutines*perG)
	}
}

func TestKeyedMutexCleansUpEntries(t *testing.T) {
	km := NewKeyedMutex()

	unlock := km.Lock("a")
	km.mu.Lock()
	if len(km.locks) != 1 {
		km.mu.Unlock()
		t.Fatalf("held key not tracked: %d entries", len(km.locks))
	}
	km.mu.Unlock()

	unlock()

	km.mu.Lock()
	defer km.mu.Unlock()
	if len(km.locks) != 0 {
		t.Fatalf("released key not cleaned up: %d entries", len(km.locks))
	}
}

func TestKeyedMutexDifferentKeysDoNotBlock(t *testing.T) {
	km := NewKeyedMutex()

	// Holding key "a" must not stop a lock on key "b" from being acquired.
	releaseA := km.Lock("a")
	defer releaseA()

	done := make(chan struct{})
	go func() {
		releaseB := km.Lock("b")
		releaseB()
		close(done)
	}()

	<-done // would deadlock/hang if distinct keys shared a lock
}
