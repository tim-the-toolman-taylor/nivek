package twitchbot

import "sync"

// chatMessageIDCacheSize is how many recent EventSub chat message_ids we
// remember. Twitch retries reuse the same id; this is sized for a busy
// multi-channel window well beyond the retry period.
const chatMessageIDCacheSize = 8192

// messageIDCache is a fixed-size ring of seen ids. seen() reports whether id
// was already claimed and claims it if not. Concurrent-safe.
type messageIDCache struct {
	mu   sync.Mutex
	ids  []string
	set  map[string]struct{}
	next int
	full bool
}

func newMessageIDCache(n int) *messageIDCache {
	if n <= 0 {
		n = chatMessageIDCacheSize
	}
	return &messageIDCache{
		ids: make([]string, n),
		set: make(map[string]struct{}, n),
	}
}

// seen reports true if id was already claimed. Empty ids cannot be deduped
// and are treated as new.
func (c *messageIDCache) seen(id string) bool {
	if id == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.set[id]; ok {
		return true
	}
	if c.full {
		delete(c.set, c.ids[c.next])
	}
	c.ids[c.next] = id
	c.set[id] = struct{}{}
	c.next++
	if c.next == len(c.ids) {
		c.next = 0
		c.full = true
	}
	return false
}
