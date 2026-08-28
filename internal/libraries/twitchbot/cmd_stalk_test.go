package twitchbot

import "testing"

func newStalkBot() *Bot {
	return &Bot{
		config: Config{BotUsername: "peanutbudderbot"},
		stalk:  make(map[string]*stalkWatch),
	}
}

func TestNormalizeStalkTarget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"Stan", "stan", true},
		{"@Stan", "stan", true},
		{"  @Dave  ", "dave", true},
		{"stan_the_man", "stan_the_man", true},
		{"", "", false},
		{"@@stan", "", false},
		{"has space", "", false},
		{"bad!name", "", false},
		{"thisloginistoolongtobevalidok", "", false},
	}
	for _, tc := range cases {
		got, ok := normalizeStalkTarget(tc.in)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("normalizeStalkTarget(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestRememberStalkMessageOnlyRecordsTarget(t *testing.T) {
	t.Parallel()
	b := newStalkBot()
	channel := "mystasuga"
	b.setStalkWatch(channel, "stan")

	b.rememberStalkMessage(channel, "joe", "Joe", "hi mom!")
	b.rememberStalkMessage(channel, "jon", "Jon", "u smell")
	b.rememberStalkMessage(channel, "dave", "Dave", "I liek turtles")
	b.rememberStalkMessage(channel, "stan", "Stan", "u REALLY smell")
	b.rememberStalkMessage(channel, "dave", "Dave", "D:")

	target, last, ok := b.stalkWatchFor(channel)
	if !ok || target != "stan" {
		t.Fatalf("watch = (%q, %v), want target stan", target, ok)
	}
	if last != "u REALLY smell" {
		t.Errorf("last = %q, want Stan's message; other chatters must not be stored", last)
	}
}

func TestRememberStalkMessageMatchesDisplayName(t *testing.T) {
	t.Parallel()
	b := newStalkBot()
	b.setStalkWatch("mystasuga", "stan")

	b.rememberStalkMessage("mystasuga", "stan_the_man", "Stan", "u REALLY smell")

	_, last, ok := b.stalkWatchFor("mystasuga")
	if !ok || last != "u REALLY smell" {
		t.Errorf("display-name match = (%q, %v), want the original text", last, ok)
	}
}

func TestRememberStalkMessageNoWatchStoresNothing(t *testing.T) {
	t.Parallel()
	b := newStalkBot()

	b.rememberStalkMessage("mystasuga", "stan", "Stan", "u REALLY smell")

	if _, _, ok := b.stalkWatchFor("mystasuga"); ok {
		t.Error("unconfigured channel should not grow a watch from chat")
	}
	if len(b.stalk) != 0 {
		t.Errorf("expected empty stalk map, got %d entries", len(b.stalk))
	}
}

func TestRememberStalkMessageSkipsStalkCommandAndBot(t *testing.T) {
	t.Parallel()
	b := newStalkBot()
	channel := "mystasuga"
	b.setStalkWatch(channel, "stan")

	b.rememberStalkMessage(channel, "stan", "Stan", "u REALLY smell")
	b.rememberStalkMessage(channel, "stan", "Stan", "!stalk")
	b.rememberStalkMessage(channel, "stan", "Stan", "!stalk set dave")
	b.rememberStalkMessage(channel, "peanutbudderbot", "peanutbudderbot", "now stalking stan")

	_, last, ok := b.stalkWatchFor(channel)
	if !ok || last != "u REALLY smell" {
		t.Errorf("after skipped messages = (%q, %v), want original chat", last, ok)
	}
}

func TestSetStalkWatchClearsPreviousLastMessage(t *testing.T) {
	t.Parallel()
	b := newStalkBot()
	channel := "mystasuga"
	b.setStalkWatch(channel, "stan")
	b.rememberStalkMessage(channel, "stan", "Stan", "u REALLY smell")

	b.setStalkWatch(channel, "dave")
	b.rememberStalkMessage(channel, "stan", "Stan", "should not stick")
	b.rememberStalkMessage(channel, "dave", "Dave", "D:")

	target, last, ok := b.stalkWatchFor(channel)
	if !ok || target != "dave" {
		t.Fatalf("watch target = (%q, %v), want dave", target, ok)
	}
	if last != "D:" {
		t.Errorf("last = %q, want Dave's message after retarget", last)
	}
}

func TestDropStalkTarget(t *testing.T) {
	t.Parallel()
	b := newStalkBot()
	b.setStalkWatch("mystasuga", "stan")
	b.rememberStalkMessage("mystasuga", "stan", "Stan", "u REALLY smell")
	b.setStalkWatch("otherchannel", "stan")
	b.rememberStalkMessage("otherchannel", "stan", "Stan", "still here")

	b.dropStalkTarget("mystasuga")

	if _, _, ok := b.stalkWatchFor("mystasuga"); ok {
		t.Error("expected mystasuga watch to be dropped")
	}
	target, last, ok := b.stalkWatchFor("otherchannel")
	if !ok || target != "stan" || last != "still here" {
		t.Errorf("other channel should be untouched, got (%q, %q, %v)", target, last, ok)
	}
}

func TestIsStalkCommand(t *testing.T) {
	t.Parallel()
	if !isStalkCommand("!stalk") || !isStalkCommand("!stalk set stan") {
		t.Fatal("expected !stalk forms to match")
	}
	if isStalkCommand("hello !stalk") || isStalkCommand("!stalker") {
		t.Fatal("did not expect substring / longer-word matches")
	}
}
