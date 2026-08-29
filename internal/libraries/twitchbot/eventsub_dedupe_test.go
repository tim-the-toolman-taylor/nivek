package twitchbot

import "testing"

func TestMessageIDCacheClaimsOnce(t *testing.T) {
	t.Parallel()
	c := newMessageIDCache(4)
	if c.seen("a") {
		t.Fatal("first claim of a should be new")
	}
	if !c.seen("a") {
		t.Fatal("second claim of a should be a duplicate")
	}
}

func TestMessageIDCacheEmptyNeverDedupes(t *testing.T) {
	t.Parallel()
	c := newMessageIDCache(4)
	if c.seen("") {
		t.Fatal("empty id should not be treated as a duplicate")
	}
	if c.seen("") {
		t.Fatal("empty id still cannot be deduped")
	}
}

func TestMessageIDCacheEvictsOldest(t *testing.T) {
	t.Parallel()
	c := newMessageIDCache(3)
	for _, id := range []string{"a", "b", "c"} {
		if c.seen(id) {
			t.Fatalf("%s should be new", id)
		}
	}
	if !c.seen("b") {
		t.Fatal("b should still be remembered")
	}
	if c.seen("d") {
		t.Fatal("d should evict the oldest (a)")
	}
	if c.seen("a") {
		t.Fatal("a should have been evicted")
	}
	if !c.seen("d") {
		t.Fatal("d should now be a duplicate")
	}
}
