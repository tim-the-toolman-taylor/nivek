package twitchbot

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/promo"
)

// handleNewPromoCommand implements !newpromo and its management subcommands,
// letting a broadcaster or mod manage recurring messages from chat:
//
//	!newpromo <interval> <message>            → create a new recurring message
//	!newpromo edit-last <interval> <message>  → replace the most recent one
//	!newpromo delete-last                     → delete the most recent one
//
// For create, the first argument is the interval (30m, 90s, 1h, or a bare
// number of minutes) and the rest is the message, posted as-is every interval
// while the channel is live. Rows are persisted via core-api and picked up by
// the promo scheduler within one poll cycle — no restart needed. Full
// management (editing any message, pausing) is on the dashboard.
func (b *Bot) handleNewPromoCommand(message *chatMessageEvent) {
	channel := message.BroadcasterUserLogin
	channelId := message.BroadcasterUserId
	username := message.ChatterUserLogin

	if !isModOrBroadcaster(message) {
		return
	}

	// Everything after the "!newpromo" token, case preserved for the message body.
	raw := strings.TrimSpace(message.Message.Text)
	args := ""
	if idx := strings.IndexAny(raw, " \t"); idx != -1 {
		args = strings.TrimSpace(raw[idx+1:])
	}

	fields := strings.Fields(args)
	if len(fields) == 0 {
		b.say(channelId, fmt.Sprintf(
			"@%s usage: !newpromo <interval e.g. 30m> <message>  ·  !newpromo edit-last <interval> <new message>  ·  !newpromo delete-last",
			username,
		))
		return
	}

	switch strings.ToLower(fields[0]) {
	case "edit-last":
		b.handlePromoEditLast(message, args, fields)
		return
	case "delete-last":
		b.handlePromoDeleteLast(message)
		return
	}

	// Default form: create. fields[0] is the interval, the remainder the message.
	if len(fields) < 2 {
		b.say(channelId, fmt.Sprintf("@%s usage: !newpromo <interval e.g. 30m> <message>", username))
		return
	}

	interval, err := parsePromoInterval(fields[0])
	if err != nil {
		b.say(channelId, fmt.Sprintf("@%s %s", username, err.Error()))
		return
	}

	promoMsg := strings.TrimSpace(args[len(fields[0]):])
	if promoMsg == "" {
		b.say(channelId, fmt.Sprintf("@%s usage: !newpromo <interval e.g. 30m> <message>", username))
		return
	}

	if errCreate := b.coreAPI.CreatePromo(channel, channelId, promoMsg, int(interval.Seconds())); errCreate != nil {
		log.Printf("[PROMO] [%s] create failed: %v", channel, errCreate)
		b.say(channelId, fmt.Sprintf("@%s couldn't save that recurring message", username))
		return
	}

	b.say(channelId, fmt.Sprintf(
		"@%s ✅ saved — I'll post that every %s while you're live. Manage your messages at https://peanutbudderbot.com",
		username, humanizeInterval(interval),
	))
	log.Printf("[PROMO] [%s] %s created a promo every %s", channel, username, humanizeInterval(interval))
}

// handlePromoEditLast implements `!newpromo edit-last <interval> <new message>`:
// it replaces the message and interval of the channel's most recently touched
// recurring message. Its enabled/paused state is preserved.
func (b *Bot) handlePromoEditLast(message *chatMessageEvent, args string, fields []string) {
	channel := message.BroadcasterUserLogin
	channelId := message.BroadcasterUserId
	username := message.ChatterUserLogin

	// fields: ["edit-last", <interval>, <message words...>]
	if len(fields) < 3 {
		b.say(channelId, fmt.Sprintf("@%s usage: !newpromo edit-last <interval e.g. 30m> <new message>", username))
		return
	}

	interval, err := parsePromoInterval(fields[1])
	if err != nil {
		b.say(channelId, fmt.Sprintf("@%s %s", username, err.Error()))
		return
	}

	// Strip the "edit-last" token, then the interval token, leaving the message.
	rest := strings.TrimSpace(args[len(fields[0]):]) // after "edit-last"
	newMsg := strings.TrimSpace(rest[len(fields[1]):])
	if newMsg == "" {
		b.say(channelId, fmt.Sprintf("@%s usage: !newpromo edit-last <interval e.g. 30m> <new message>", username))
		return
	}

	found, errEdit := b.coreAPI.EditLastPromo(channel, newMsg, int(interval.Seconds()))
	if errEdit != nil {
		log.Printf("[PROMO] [%s] edit-last failed: %v", channel, errEdit)
		b.say(channelId, fmt.Sprintf("@%s couldn't update your latest recurring message", username))
		return
	}
	if !found {
		b.say(channelId, fmt.Sprintf("@%s you have no recurring messages to edit — make one with !newpromo <interval> <message>", username))
		return
	}

	b.say(channelId, fmt.Sprintf("@%s ✅ updated your latest recurring message — now every %s", username, humanizeInterval(interval)))
	log.Printf("[PROMO] [%s] %s edited last promo", channel, username)
}

// handlePromoDeleteLast implements `!newpromo delete-last`: it removes the
// channel's most recently touched recurring message.
func (b *Bot) handlePromoDeleteLast(message *chatMessageEvent) {
	channel := message.BroadcasterUserLogin
	channelId := message.BroadcasterUserId
	username := message.ChatterUserLogin

	found, err := b.coreAPI.DeleteLastPromo(channel)
	if err != nil {
		log.Printf("[PROMO] [%s] delete-last failed: %v", channel, err)
		b.say(channelId, fmt.Sprintf("@%s couldn't delete your latest recurring message", username))
		return
	}
	if !found {
		b.say(channelId, fmt.Sprintf("@%s you have no recurring messages to delete", username))
		return
	}

	b.say(channelId, fmt.Sprintf("@%s 🗑️ deleted your latest recurring message", username))
	log.Printf("[PROMO] [%s] %s deleted last promo", channel, username)
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
