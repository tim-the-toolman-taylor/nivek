package twitchbot

import "testing"

func newLastChatBot() *Bot {
	return &Bot{
		config:        Config{BotUsername: "peanutbudderbot"},
		lastChat:      make(map[string]map[string]string),
		lastChatAlias: make(map[string]map[string]string),
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

func TestLastChatQuotesMostRecentPerChatter(t *testing.T) {
	t.Parallel()
	b := newLastChatBot()
	channel := "mystasuga"

	b.rememberChat(channel, "joe", "Joe", "hi mom!")
	b.rememberChat(channel, "jon", "Jon", "u smell")
	b.rememberChat(channel, "dave", "Dave", "I liek turtles")
	b.rememberChat(channel, "stan", "Stan", "u REALLY smell")
	b.rememberChat(channel, "dave", "Dave", "D:")

	if got, ok := b.lastChatFrom(channel, "stan"); !ok || got != "u REALLY smell" {
		t.Errorf("stalk Stan = (%q, %v), want %q", got, ok, "u REALLY smell")
	}
	if got, ok := b.lastChatFrom(channel, "Dave"); !ok || got != "D:" {
		t.Errorf("stalk Dave = (%q, %v), want %q", got, ok, "D:")
	}
	if got, ok := b.lastChatFrom(channel, "@joe"); !ok || got != "hi mom!" {
		t.Errorf("stalk @joe = (%q, %v), want %q", got, ok, "hi mom!")
	}
}

func TestLastChatMatchesDisplayNameAlias(t *testing.T) {
	t.Parallel()
	b := newLastChatBot()

	b.rememberChat("mystasuga", "stan_the_man", "Stan", "u REALLY smell")

	got, ok := b.lastChatFrom("mystasuga", "Stan")
	if !ok || got != "u REALLY smell" {
		t.Errorf("display-name lookup = (%q, %v), want the original text", got, ok)
	}
}

func TestLastChatSkipsStalkCommandAndBot(t *testing.T) {
	t.Parallel()
	b := newLastChatBot()
	channel := "mystasuga"

	b.rememberChat(channel, "stan", "Stan", "u REALLY smell")
	b.rememberChat(channel, "stan", "Stan", "!stalk")
	b.rememberChat(channel, "stan", "Stan", "!stalk set dave")
	b.rememberChat(channel, "peanutbudderbot", "peanutbudderbot", "now stalking stan")

	got, ok := b.lastChatFrom(channel, "stan")
	if !ok || got != "u REALLY smell" {
		t.Errorf("after skipped messages = (%q, %v), want original chat", got, ok)
	}
	if _, ok := b.lastChatFrom(channel, "peanutbudderbot"); ok {
		t.Error("bot's own messages should not be stored")
	}
}

func TestDropLastChat(t *testing.T) {
	t.Parallel()
	b := newLastChatBot()

	b.rememberChat("mystasuga", "stan", "Stan", "u REALLY smell")
	b.rememberChat("otherchannel", "stan", "Stan", "still here")
	b.dropLastChat("mystasuga")

	if _, ok := b.lastChatFrom("mystasuga", "stan"); ok {
		t.Error("expected mystasuga history to be dropped")
	}
	got, ok := b.lastChatFrom("otherchannel", "stan")
	if !ok || got != "still here" {
		t.Errorf("other channel should be untouched, got (%q, %v)", got, ok)
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
