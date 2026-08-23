package twitchbot

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

// handleDadCommand implements !dad and its management subcommands:
//
//	!dad                → post a random response (globals + this message.BroadcasterUserId's own)
//	!dad add <text>     → add a response for this message.BroadcasterUserId (broadcaster/mods only)
//	!dad remove <id>    → remove one of this message.BroadcasterUserId's responses (broadcaster/mods only)
//
// Responses, their per-message.BroadcasterUserId scoping, and usage counts all live in the DB
// behind core-api; the bot holds none of it.
func (b *Bot) handleDadCommand(message *chatMessageEvent) {
	username := message.ChatterUserLogin

	// Everything after the "!dad" token, case-preserved (the command token's own
	// case doesn't matter — dispatch already matched it case-insensitively).
	raw := strings.TrimSpace(message.Message.Text)
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
			resp := fmt.Sprintf("@%s usage: !dad add <response>", username)
			b.say(&message.BroadcasterUserId, &resp)
			return
		}
		if err := b.coreAPI.DadAdd(message.BroadcasterUserId, text); err != nil {
			log.Printf("[DAD] [%s] add failed: %v", message.BroadcasterUserId, err)
			resp := fmt.Sprintf("@%s couldn't add that response", username)
			b.say(&message.BroadcasterUserId, &resp)
			return
		}

		resp := fmt.Sprintf("@%s added a new !dad response", username)
		b.say(&message.BroadcasterUserId, &resp)

	case "remove":
		if !canManageDad(message) {
			return
		}
		if len(fields) < 2 {
			resp := fmt.Sprintf("@%s usage: !dad remove <id>", username)
			b.say(&message.BroadcasterUserId, &resp)
			return
		}
		id, err := strconv.Atoi(fields[1])
		if err != nil {
			resp := fmt.Sprintf("@%s usage: !dad remove <id>", username)
			b.say(&message.BroadcasterUserId, &resp)
			return
		}
		if err := b.coreAPI.DadRemove(message.BroadcasterUserId, id); err != nil {
			log.Printf("[DAD] [%s] remove failed: %v", message.BroadcasterUserId, err)
			resp := fmt.Sprintf("@%s couldn't remove response #%d", username, id)
			b.say(&message.BroadcasterUserId, &resp)
			return
		}

		resp := fmt.Sprintf("@%s removed !dad response #%d", username, id)
		b.say(&message.BroadcasterUserId, &resp)

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
func (b *Bot) rollDad(message *chatMessageEvent) {
	switch b.checkDadLimit(message.BroadcasterUserLogin, message.ChatterUserLogin) {
	case dadAllow:
		b.sayRandomDad(message.BroadcasterUserId, message.BroadcasterUserLogin)
		// Persist the counted roll so a restart mid-stream doesn't reset it.
		go b.persistDadRoll(message.BroadcasterUserId, message.ChatterUserLogin)
	case dadReject:
		resp := randomDadReject()
		b.say(&message.BroadcasterUserId, &resp)
		// The limit-crossing roll is still counted; persist it too.
		go b.persistDadRoll(message.BroadcasterUserId, message.ChatterUserLogin)
	case dadSilent:
		// already warned this chatter this stream — say nothing, and don't
		// persist: over-limit rolls never touch the DB.
	}
}

func (b *Bot) sayRandomDad(broadcasterUserId, broadcasterUserLogin string) {
	response, err := b.coreAPI.DadRandom(broadcasterUserLogin)
	if err != nil {
		log.Printf("[DAD] [%s] random failed: %v", broadcasterUserLogin, err)
		return
	}
	if response != "" {
		b.say(&broadcasterUserId, &response)
	}
}

// canManageDad limits !dad add/remove to the broadcaster and message.BroadcasterUserId mods.
func canManageDad(message *chatMessageEvent) bool {
	return isModOrBroadcaster(message.BroadcasterUserId, message.ChatterUserId, message.Badges)
}
