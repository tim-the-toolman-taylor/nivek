package twitchbot

import (
	"testing"

	"github.com/gempir/go-twitch-irc/v4"
)

func TestIsBotIRCUser(t *testing.T) {
	t.Parallel()
	b := &Bot{config: Config{BotUsername: "peanutbudderbot", BotId: "1322716097"}}

	if !b.isBotIRCUser(twitch.PrivateMessage{User: twitch.User{ID: "1322716097", Name: "someone"}}) {
		t.Fatal("bot user id should be skipped")
	}
	if !b.isBotIRCUser(twitch.PrivateMessage{User: twitch.User{ID: "1", Name: "PeanutBudderBot"}}) {
		t.Fatal("bot login should be skipped case-insensitively")
	}
	if !b.isBotIRCUser(twitch.PrivateMessage{User: twitch.User{ID: "1", Name: "x", DisplayName: "peanutbudderbot"}}) {
		t.Fatal("bot display name should be skipped")
	}
	if b.isBotIRCUser(twitch.PrivateMessage{User: twitch.User{ID: "99", Name: "alice"}}) {
		t.Fatal("other chatters should not be treated as the bot")
	}
}

func TestIsChatReadNudge(t *testing.T) {
	t.Parallel()
	b := &Bot{config: Config{BotUsername: "peanutbudderbot"}}
	nudge := b.chatReadNudgeText()
	if !b.isChatReadNudge(nudge) {
		t.Fatal("exact nudge text should be skipped")
	}
	if !b.isChatReadNudge("  " + nudge + "  ") {
		t.Fatal("padded nudge text should be skipped")
	}
	if b.isChatReadNudge("hello") {
		t.Fatal("ordinary chat should not be treated as the nudge")
	}
}
