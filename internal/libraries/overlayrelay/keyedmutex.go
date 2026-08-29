package overlayrelay

import "sync"

// KeyedMutex is a set of mutexes addressed by string key, so callers can
// serialise work per key without a global lock.
//
// The overlay ingest path needs this: two EventSub deliveries for the same
// broadcaster commit and then push to the live outbox from independent HTTP
// handlers. The database keeps seq assignment gapless, but nothing otherwise
// stops one handler pushing seq N+1 before another pushes seq N -- and an
// overlay that persists the last seq it processed as its reconnect cursor would
// then skip the lagging event. Holding the broadcaster's key across append+push
// keeps live frames in seq order, matching that cursor protocol.
//
// In-process only, which matches the single-instance assumption the connection
// registry already documents; more than one core-api would need a shared lock.
type KeyedMutex struct {
	mu    sync.Mutex
	locks map[string]*refLock
}

type refLock struct {
	mu   sync.Mutex
	refs int
}

func NewKeyedMutex() *KeyedMutex {
	return &KeyedMutex{locks: make(map[string]*refLock)}
}

// Lock acquires the mutex for key and returns the function that releases it. The
// per-key entry is reference-counted and dropped once no caller holds or waits
// on it, so the map cannot grow without bound.
func (k *KeyedMutex) Lock(key string) func() {
	k.mu.Lock()
	rl, ok := k.locks[key]
	if !ok {
		rl = &refLock{}
		k.locks[key] = rl
	}
	rl.refs++
	k.mu.Unlock()

	rl.mu.Lock()

	return func() {
		rl.mu.Unlock()
		k.mu.Lock()
		rl.refs--
		if rl.refs == 0 {
			delete(k.locks, key)
		}
		k.mu.Unlock()
	}
}
