package twitchbot

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gempir/go-twitch-irc/v4"
)

// handleDadCommand implements !dad and its management subcommands:
//
//	!dad                → post a random response (globals + this channel's own)
//	!dad add <text>     → add a response for this channel (broadcaster/mods only)
//	!dad remove <id>    → remove one of this channel's responses (broadcaster/mods only)
//
// Responses, their per-channel scoping, and usage counts all live in the DB
// behind core-api; the bot holds none of it.
func (b *Bot) handleDadCommand(message *twitch.PrivateMessage) {
	channel := message.Channel
	username := message.User.Name

	// Everything after the "!dad" token, case-preserved (the command token's own
	// case doesn't matter — dispatch already matched it case-insensitively).
	raw := strings.TrimSpace(message.Message)
	args := ""
	if idx := strings.IndexAny(raw, " \t"); idx != -1 {
		args = strings.TrimSpace(raw[idx+1:])
	}
	fields := strings.Fields(args)

	if len(fields) == 0 {
		b.rollDad(message)
		return
	}

	switch strings.ToLower(fields[0]) {
	case "add":
		if !canManageDad(message) {
			return
		}
		text := strings.TrimSpace(args[len(fields[0]):])
		if text == "" {
			b.say(channel, fmt.Sprintf("@%s usage: !dad add <response>", username))
			return
		}
		if err := b.coreAPI.DadAdd(channel, text); err != nil {
			log.Printf("[DAD] [%s] add failed: %v", channel, err)
			b.say(channel, fmt.Sprintf("@%s couldn't add that response", username))
			return
		}
		b.say(channel, fmt.Sprintf("@%s added a new !dad response", username))

	case "remove":
		if !canManageDad(message) {
			return
		}
		if len(fields) < 2 {
			b.say(channel, fmt.Sprintf("@%s usage: !dad remove <id>", username))
			return
		}
		id, err := strconv.Atoi(fields[1])
		if err != nil {
			b.say(channel, fmt.Sprintf("@%s usage: !dad remove <id>", username))
			return
		}
		if err := b.coreAPI.DadRemove(channel, id); err != nil {
			log.Printf("[DAD] [%s] remove failed: %v", channel, err)
			b.say(channel, fmt.Sprintf("@%s couldn't remove response #%d", username, id))
			return
		}
		b.say(channel, fmt.Sprintf("@%s removed !dad response #%d", username, id))

	default:
		// Any other trailing text is treated as a plain !dad roll.
		b.rollDad(message)
	}
}

// rollDad handles an unprivileged "!dad" roll under the per-stream, per-chatter
// limit: below the cap it serves a real response; on the first over-cap roll it
// sends a single dad-flavored reject; after that it stays silent for that chatter
// until the next stream. The limit check is entirely in-process (see
// dad_limit.go), so an over-limit roll makes no core-api call.
func (b *Bot) rollDad(message *twitch.PrivateMessage) {
	switch b.checkDadLimit(message.Channel, message.User.Name) {
	case dadAllow:
		b.sayRandomDad(message.Channel)
	case dadReject:
		b.say(message.Channel, randomDadReject())
	case dadSilent:
		// already warned this chatter this stream — say nothing
	}
}

func (b *Bot) sayRandomDad(channel string) {
	response, err := b.coreAPI.DadRandom(channel)
	if err != nil {
		log.Printf("[DAD] [%s] random failed: %v", channel, err)
		return
	}
	if response != "" {
		b.say(channel, response)
	}
}

// canManageDad limits !dad add/remove to the broadcaster and channel mods.
func canManageDad(message *twitch.PrivateMessage) bool {
	return isModOrBroadcaster(message)
}
