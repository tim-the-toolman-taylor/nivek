package twitchbot

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/gempir/go-twitch-irc/v4"
)

// Twitch logins are 1–25 chars of a-z, 0-9, underscore. Display names match the
// same character set (we lowercase before storing/matching).
var stalkLoginRe = regexp.MustCompile(`^[a-z0-9_]{1,25}$`)

const stalkUsage = "usage: !stalk  ·  !stalk set <username>  ·  !stalk clear"

// handleStalkCommand implements !stalk:
//
//	!stalk                 → quote the last message from the configured target
//	!stalk set <username>  → mod/broadcaster: persist who this channel stalks
//	!stalk <username>      → mod/broadcaster shorthand for set
//	!stalk clear           → mod/broadcaster: remove the target
//
// Anyone can run the no-arg form. The quoted reply is the target's last chat
// text verbatim — no attribution prefix. The target is persisted via core-api;
// last-message lookup is in-memory (see rememberChat).
func (b *Bot) handleStalkCommand(message *twitch.PrivateMessage) {
	channel := message.Channel
	username := message.User.Name

	raw := strings.TrimSpace(message.Message)
	args := ""
	if idx := strings.IndexAny(raw, " \t"); idx != -1 {
		args = strings.TrimSpace(raw[idx+1:])
	}
	fields := strings.Fields(args)

	if len(fields) == 0 {
		b.quoteStalkTarget(channel)
		return
	}

	switch strings.ToLower(fields[0]) {
	case "set":
		if !isModOrBroadcaster(message) {
			b.say(channel, fmt.Sprintf("@%s only a mod or the broadcaster can pick who to stalk", username))
			return
		}
		if len(fields) < 2 {
			b.say(channel, fmt.Sprintf("@%s %s", username, stalkUsage))
			return
		}
		b.setStalkTarget(channel, username, fields[1])
		return
	case "clear", "unset":
		if !isModOrBroadcaster(message) {
			b.say(channel, fmt.Sprintf("@%s only a mod or the broadcaster can pick who to stalk", username))
			return
		}
		b.clearStalkTarget(channel, username)
		return
	}

	// Mod shorthand: `!stalk Stan` is `!stalk set Stan`. Viewers with extra
	// words still just run the quote — they can't reconfigure by accident.
	if isModOrBroadcaster(message) && len(fields) == 1 {
		b.setStalkTarget(channel, username, fields[0])
		return
	}

	b.quoteStalkTarget(channel)
}

func (b *Bot) setStalkTarget(channel, setBy, rawTarget string) {
	target, ok := normalizeStalkTarget(rawTarget)
	if !ok {
		b.say(channel, fmt.Sprintf("@%s that doesn't look like a username", setBy))
		return
	}

	if err := b.coreAPI.SetStalkTarget(channel, target, setBy); err != nil {
		log.Printf("[STALK] [%s] set failed: %v", channel, err)
		b.say(channel, fmt.Sprintf("@%s couldn't save that stalk target", setBy))
		return
	}

	b.say(channel, fmt.Sprintf("now stalking %s", target))
	log.Printf("[STALK] [%s] %s set target to %s", channel, setBy, target)
}

func (b *Bot) clearStalkTarget(channel, username string) {
	found, err := b.coreAPI.ClearStalkTarget(channel)
	if err != nil {
		log.Printf("[STALK] [%s] clear failed: %v", channel, err)
		b.say(channel, fmt.Sprintf("@%s couldn't clear the stalk target", username))
		return
	}
	if !found {
		b.say(channel, fmt.Sprintf("@%s nobody was being stalked", username))
		return
	}
	b.say(channel, "stopped stalking")
	log.Printf("[STALK] [%s] %s cleared target", channel, username)
}

func (b *Bot) quoteStalkTarget(channel string) {
	target, found, err := b.coreAPI.GetStalkTarget(channel)
	if err != nil {
		log.Printf("[STALK] [%s] get target failed: %v", channel, err)
		b.say(channel, "couldn't look up who we're stalking")
		return
	}
	if !found || target == "" {
		b.say(channel, "nobody is being stalked yet — a mod can pick someone with !stalk set <username>")
		return
	}

	text, ok := b.lastChatFrom(channel, target)
	if !ok {
		b.say(channel, fmt.Sprintf("haven't seen a message from %s yet", target))
		return
	}
	b.say(channel, text)
}

// normalizeStalkTarget strips a leading @, lowercases, and checks the result
// looks like a Twitch login/display name. ok is false for empty/junk input.
func normalizeStalkTarget(raw string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, "@")
	s = strings.TrimSpace(s)
	if !stalkLoginRe.MatchString(s) {
		return "", false
	}
	return s, true
}

func isStalkCommand(msg string) bool {
	return msg == "!stalk" || strings.HasPrefix(msg, "!stalk ")
}

// rememberChat stores the chatter's latest message in this channel so !stalk
// can quote it. Skips empty text, the bot's own messages, and !stalk itself
// (so running the command doesn't overwrite the target's last real chat).
func (b *Bot) rememberChat(channel, login, display, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if isStalkCommand(strings.ToLower(text)) {
		return
	}

	channel = strings.ToLower(strings.TrimSpace(channel))
	login = strings.ToLower(strings.TrimSpace(login))
	if channel == "" || login == "" {
		return
	}
	if strings.EqualFold(login, b.config.BotUsername) {
		return
	}

	displayKey := strings.ToLower(strings.TrimSpace(display))

	b.lastChatMu.Lock()
	defer b.lastChatMu.Unlock()
	if b.lastChat[channel] == nil {
		b.lastChat[channel] = make(map[string]string)
		b.lastChatAlias[channel] = make(map[string]string)
	}
	b.lastChat[channel][login] = text
	b.lastChatAlias[channel][login] = login
	if displayKey != "" {
		b.lastChatAlias[channel][displayKey] = login
	}
}

// lastChatFrom returns the last remembered message from `who` in this channel.
// `who` may be a login or a display name; both are matched case-insensitively.
func (b *Bot) lastChatFrom(channel, who string) (string, bool) {
	channel = strings.ToLower(strings.TrimSpace(channel))
	who = strings.ToLower(strings.TrimSpace(who))
	who = strings.TrimPrefix(who, "@")
	if channel == "" || who == "" {
		return "", false
	}

	b.lastChatMu.Lock()
	defer b.lastChatMu.Unlock()
	aliases := b.lastChatAlias[channel]
	msgs := b.lastChat[channel]
	if msgs == nil {
		return "", false
	}
	login := who
	if aliases != nil {
		if resolved, ok := aliases[who]; ok {
			login = resolved
		}
	}
	text, ok := msgs[login]
	return text, ok
}

// dropLastChat evicts a channel's last-message map. Called on stream.offline
// so the in-memory history stays bounded to currently-live channels.
func (b *Bot) dropLastChat(login string) {
	key := strings.ToLower(login)
	b.lastChatMu.Lock()
	delete(b.lastChat, key)
	delete(b.lastChatAlias, key)
	b.lastChatMu.Unlock()
}
