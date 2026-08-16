package twitchbot

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gempir/go-twitch-irc/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/promo"
)

// handleNewPromoCommand implements !newpromo, letting a broadcaster or mod set a
// recurring message from chat:
//
//	!newpromo 30m Join my discord! https://discord.gg/...
//
// The first argument is the interval (30m, 90s, 1h, or a bare number of
// minutes); the rest is the message, posted as-is every interval while the
// channel is live. The row is persisted via core-api and picked up by the promo
// scheduler within one poll cycle — no restart needed. Editing/deleting existing
// promos is done from the dashboard.
func (b *Bot) handleNewPromoCommand(message *twitch.PrivateMessage) {
	channel := message.Channel
	username := message.User.Name

	if !isModOrBroadcaster(message) {
		return
	}

	// Everything after the "!newpromo" token, case preserved for the message body.
	raw := strings.TrimSpace(message.Message)
	args := ""
	if idx := strings.IndexAny(raw, " \t"); idx != -1 {
		args = strings.TrimSpace(raw[idx+1:])
	}

	fields := strings.Fields(args)
	if len(fields) < 2 {
		b.say(channel, fmt.Sprintf("@%s usage: !newpromo <interval e.g. 30m> <message>", username))
		return
	}

	interval, err := parsePromoInterval(fields[0])
	if err != nil {
		b.say(channel, fmt.Sprintf("@%s %s", username, err.Error()))
		return
	}

	// The message is the remainder after the interval token.
	promoMsg := strings.TrimSpace(args[len(fields[0]):])
	if promoMsg == "" {
		b.say(channel, fmt.Sprintf("@%s usage: !newpromo <interval e.g. 30m> <message>", username))
		return
	}

	if errCreate := b.coreAPI.CreatePromo(channel, promoMsg, int(interval.Seconds())); errCreate != nil {
		log.Printf("[PROMO] [%s] create failed: %v", channel, errCreate)
		b.say(channel, fmt.Sprintf("@%s couldn't save that recurring message", username))
		return
	}

	b.say(channel, fmt.Sprintf(
		"@%s ✅ saved — I'll post that every %s while you're live. Manage your messages at https://peanutbudderbot.com",
		username, humanizeInterval(interval),
	))
	log.Printf("[PROMO] [%s] %s created a promo every %s", channel, username, humanizeInterval(interval))
}

// parsePromoInterval reads an interval token. A bare integer is minutes
// (streamer-friendly: "30" == 30m); anything else goes through
// time.ParseDuration ("30m", "90s", "1h", "1h30m"). The result is bounded to
// the promo service's min/max.
func parsePromoInterval(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)

	if n, err := strconv.Atoi(s); err == nil {
		return validatePromoDuration(time.Duration(n) * time.Minute)
	}

	d, err := time.ParseDuration(strings.ToLower(s))
	if err != nil {
		return 0, fmt.Errorf("couldn't read %q as a time — try 30m, 90s, or 1h", s)
	}
	return validatePromoDuration(d)
}

func validatePromoDuration(d time.Duration) (time.Duration, error) {
	if d < time.Duration(promo.MinIntervalSeconds)*time.Second {
		return 0, fmt.Errorf("interval must be at least 1 minute")
	}
	if d > time.Duration(promo.MaxIntervalSeconds)*time.Second {
		return 0, fmt.Errorf("interval must be 24 hours or less")
	}
	return d, nil
}

// humanizeInterval renders a duration back to a compact, chat-friendly string
// for the confirmation message (e.g. 30m, 1h, 90s).
func humanizeInterval(d time.Duration) string {
	d = d.Round(time.Second)
	switch {
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	default:
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
}
