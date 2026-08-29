package twitchbot

import (
	"log"
	"strings"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/commands"
)

// Per-channel custom ("channel"-scoped) commands are loaded on demand rather
// than once at boot: the set is pulled when a channel goes live (stream.online
// webhook, or at boot for channels already live) and dropped when it goes
// offline. This keeps the dispatch map small and the in-memory data fresh for
// the current stream — a broadcaster who edits a command mid-stream picks it up
// on their next go-live, matching how the bot already treats a live session as
// the unit of state (auto-shout, !dad counters).

// loadCustomCommands fetches the channel's enabled custom commands from core-api
// (keyed by Twitch id) and stores them for dispatch under the lowercased login,
// which is what handleMessage matches on. Safe to call repeatedly — it replaces
// the channel's set wholesale, so a re-fire (e.g. a duplicate stream.online)
// just refreshes it. Intended to run in its own goroutine off the webhook/boot
// path since it makes a network call.
func (b *Bot) loadCustomCommands(twitchID, login string) {
	rows, err := b.coreAPI.GetChannelCommands(twitchID)
	if err != nil {
		log.Printf("[CUSTOM-CMD] failed to load custom commands for %s (%s): %s", login, twitchID, err.Error())
		return
	}

	set := make(map[string]commands.Commands, len(rows))
	for _, row := range rows {
		// A channel row without a template can't be answered (only global
		// builtins carry a compiled handler), so skip it rather than register a
		// trigger that would silently do nothing.
		if row.ResponseTmpl == nil {
			continue
		}
		set[strings.ToLower(row.Trigger)] = row
	}

	key := strings.ToLower(login)
	b.customMu.Lock()
	if len(set) == 0 {
		delete(b.customCommands, key)
	} else {
		b.customCommands[key] = set
	}
	b.customMu.Unlock()

	log.Printf("[CUSTOM-CMD] loaded %d custom command(s) for %s", len(set), login)
}

// dropCustomCommands evicts a channel's custom command set. Called on
// stream.offline so an offline channel's triggers stop responding and the map
// stays bounded to currently-live channels.
func (b *Bot) dropCustomCommands(login string) {
	key := strings.ToLower(login)
	b.customMu.Lock()
	delete(b.customCommands, key)
	b.customMu.Unlock()
}

// customCommandFor looks up a channel's custom command by trigger (both keys are
// lowercased). ok is false when the channel has no custom set or no such
// trigger.
func (b *Bot) customCommandFor(login, trigger string) (commands.Commands, bool) {
	b.customMu.Lock()
	defer b.customMu.Unlock()
	set, ok := b.customCommands[strings.ToLower(login)]
	if !ok {
		return commands.Commands{}, false
	}
	cmd, ok := set[strings.ToLower(trigger)]
	return cmd, ok
}

// meetsMinRole reports whether the message's author clears a command's min_role.
// Roles are cumulative (a broadcaster clears every tier). "everyone" and any
// unrecognized value are open to all, matching the column's default.
func meetsMinRole(message *chatMessageEvent, minRole string) bool {
	isBroadcaster := message.BroadcasterUserId == message.ChatterUserId
	isModerator   := false
	isSubscriber  := false
	isVip         := false

	for _, badge := range message.Badges {
		if badge.SetId == "moderator" {
			isModerator = true
		}

		if badge.SetId == "subscriber" {
			isSubscriber = true
		}

		if badge.SetId == "vip" {
			isVip = true
		}
	}

	switch minRole {
	case "broadcaster":
		return isBroadcaster
	case "mod":
		if isBroadcaster || isModerator {
			return true
		}

		return false
	case "vip":
		return isBroadcaster || isModerator || isVip
	case "sub":
		if isBroadcaster || isModerator || isSubscriber {
			return true
		}

		return false
	default: // "everyone" or anything unexpected
		return true
	}
}
