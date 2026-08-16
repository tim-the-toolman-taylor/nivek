package twitchbot

import (
	"fmt"
	"log"
	"strings"

	"github.com/gempir/go-twitch-irc/v4"
)

// commandsPageURL is the public, no-login page that lists every command and
// action the bot supports. !pbcommands just points chat at it rather than
// dumping the whole list inline. It's the bot's own domain, so it won't trip
// link-spam heuristics — the same pattern the !DF welcome and !joinme promo
// messages already use.
const commandsPageURL = "https://peanutbudderbot.com/commands"

// handlePbCommandsCommand replies with a link to the public commands page, and
// gives broadcasters/mods two chat shortcuts for managing this channel's most
// recently touched recurring message:
//
//	!pbcommands edit-last <interval> <new message>  → replace it
//	!pbcommands delete-last                         → remove it
//
// Anything else (including bare !pbcommands) just links the commands page.
func (b *Bot) handlePbCommandsCommand(message *twitch.PrivateMessage) {
	// Everything after the "!pbcommands" token, case preserved for the message body.
	raw := strings.TrimSpace(message.Message)
	args := ""
	if idx := strings.IndexAny(raw, " \t"); idx != -1 {
		args = strings.TrimSpace(raw[idx+1:])
	}
	fields := strings.Fields(args)

	if len(fields) == 0 {
		b.sayCommandsLink(message)
		return
	}

	switch strings.ToLower(fields[0]) {
	case "edit-last":
		if !isModOrBroadcaster(message) {
			return
		}
		b.handlePromoEditLast(message, args, fields)
	case "delete-last":
		if !isModOrBroadcaster(message) {
			return
		}
		b.handlePromoDeleteLast(message)
	default:
		b.sayCommandsLink(message)
	}
}

func (b *Bot) sayCommandsLink(message *twitch.PrivateMessage) {
	b.say(message.Channel, "📜 All my commands and actions are listed here: "+commandsPageURL)
	log.Printf("[PBCOMMANDS] [%s] %s", message.Channel, message.User.Name)
}

// handlePromoEditLast implements `!pbcommands edit-last <interval> <new message>`:
// it replaces the message and interval of the channel's most recently touched
// recurring message. Its enabled/paused state is preserved.
func (b *Bot) handlePromoEditLast(message *twitch.PrivateMessage, args string, fields []string) {
	channel := message.Channel
	username := message.User.Name

	if len(fields) < 3 {
		b.say(channel, fmt.Sprintf("@%s usage: !pbcommands edit-last <interval e.g. 30m> <new message>", username))
		return
	}

	interval, err := parsePromoInterval(fields[1])
	if err != nil {
		b.say(channel, fmt.Sprintf("@%s %s", username, err.Error()))
		return
	}

	// Strip the "edit-last" token, then the interval token, leaving the message.
	rest := strings.TrimSpace(args[len(fields[0]):]) // after "edit-last"
	newMsg := strings.TrimSpace(rest[len(fields[1]):])
	if newMsg == "" {
		b.say(channel, fmt.Sprintf("@%s usage: !pbcommands edit-last <interval e.g. 30m> <new message>", username))
		return
	}

	found, errEdit := b.coreAPI.EditLastPromo(channel, newMsg, int(interval.Seconds()))
	if errEdit != nil {
		log.Printf("[PBCOMMANDS] [%s] edit-last failed: %v", channel, errEdit)
		b.say(channel, fmt.Sprintf("@%s couldn't update your latest recurring message", username))
		return
	}
	if !found {
		b.say(channel, fmt.Sprintf("@%s you have no recurring messages to edit — make one with !newpromo", username))
		return
	}

	b.say(channel, fmt.Sprintf("@%s ✅ updated your latest recurring message — now every %s", username, humanizeInterval(interval)))
	log.Printf("[PBCOMMANDS] [%s] %s edited last promo", channel, username)
}

// handlePromoDeleteLast implements `!pbcommands delete-last`: it removes the
// channel's most recently touched recurring message.
func (b *Bot) handlePromoDeleteLast(message *twitch.PrivateMessage) {
	channel := message.Channel
	username := message.User.Name

	found, err := b.coreAPI.DeleteLastPromo(channel)
	if err != nil {
		log.Printf("[PBCOMMANDS] [%s] delete-last failed: %v", channel, err)
		b.say(channel, fmt.Sprintf("@%s couldn't delete your latest recurring message", username))
		return
	}
	if !found {
		b.say(channel, fmt.Sprintf("@%s you have no recurring messages to delete", username))
		return
	}

	b.say(channel, fmt.Sprintf("@%s 🗑️ deleted your latest recurring message", username))
	log.Printf("[PBCOMMANDS] [%s] %s deleted last promo", channel, username)
}
