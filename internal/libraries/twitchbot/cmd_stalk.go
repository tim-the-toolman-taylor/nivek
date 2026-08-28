package twitchbot

import (
	"fmt"
	"log"
	"strings"

	"github.com/gempir/go-twitch-irc/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/stalk"
)

const stalkUsage = "usage: !stalk  ·  !stalk set <username>  ·  !stalk clear"

// handleStalkCommand implements !stalk:
//
//	!stalk                 → quote the last message from the configured target
//	!stalk set <username>  → mod/broadcaster: persist who this channel stalks
//	!stalk <username>      → mod/broadcaster shorthand for set
//	!stalk clear           → mod/broadcaster: remove the target
//
// Anyone can run the no-arg form. The quoted reply is the target's last chat
// text verbatim — no attribution prefix. The target is persisted via core-api
// and loaded into memory on go-live (like custom commands); handleMessage only
// records chat from that one chatter.
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

	// Apply immediately so this stream starts recording the new target without
	// waiting for the next go-live. Custom commands can't be edited from chat,
	// so they wait; !stalk set is the chat-side edit.
	b.setStalkWatch(channel, target)
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
	b.dropStalkTarget(channel)
	if !found {
		b.say(channel, fmt.Sprintf("@%s nobody was being stalked", username))
		return
	}
	b.say(channel, "stopped stalking")
	log.Printf("[STALK] [%s] %s cleared target", channel, username)
}

func (b *Bot) quoteStalkTarget(channel string) {
	// Refresh from core-api so a dashboard edit takes effect on the next !stalk
	// without waiting for a go-live, and so a reboot can quote last_message.
	apiTarget, apiLast, found, err := b.coreAPI.GetStalkTarget(channel)
	if err != nil {
		log.Printf("[STALK] [%s] get target failed: %v", channel, err)
		b.say(channel, "couldn't look up who we're stalking")
		return
	}
	if !found || apiTarget == "" {
		b.dropStalkTarget(channel)
		b.say(channel, "nobody is being stalked yet — a mod can pick someone with !stalk set <username>")
		return
	}

	b.setStalkWatch(channel, apiTarget)
	b.hydrateStalkLast(channel, apiLast)

	target, last, ok := b.stalkWatchFor(channel)
	if !ok {
		b.say(channel, "nobody is being stalked yet — a mod can pick someone with !stalk set <username>")
		return
	}
	if last == "" {
		b.say(channel, fmt.Sprintf("haven't seen a message from %s yet", target))
		return
	}

	b.say(channel, fmt.Sprintf("%s said: %s", target, last))
}

func normalizeStalkTarget(raw string) (string, bool) {
	return stalk.NormalizeTarget(raw)
}

func isStalkCommand(msg string) bool {
	return msg == "!stalk" || strings.HasPrefix(msg, "!stalk ")
}

// stalkWatch is the live-session state for one channel's !stalk: who we're
// watching, and that chatter's last message this stream (empty until they talk).
type stalkWatch struct {
	target string
	last   string
}

// loadStalkTarget fetches the channel's configured target from core-api and
// holds it in memory for this stream — the same live-session pattern as custom
// commands. If the channel has no target, nothing is stored and handleMessage
// will not record anyone. Intended to run in its own goroutine off the
// webhook/boot path since it makes a network call.
func (b *Bot) loadStalkTarget(login string) {
	login = strings.ToLower(strings.TrimSpace(login))
	if login == "" {
		return
	}

	target, last, found, err := b.coreAPI.GetStalkTarget(login)
	if err != nil {
		log.Printf("[STALK] failed to load target for %s: %s", login, err.Error())
		return
	}
	if !found || target == "" {
		// Don't drop here: a !stalk set can land while this GET is in flight,
		// and wiping memory would throw away a target just written to the DB.
		// Unconfigured channels simply never get a watch (offline already
		// dropped the previous stream; !stalk clear drops on its own).
		log.Printf("[STALK] no target configured for %s", login)
		return
	}

	b.setStalkWatch(login, target)
	b.hydrateStalkLast(login, last)
	log.Printf("[STALK] loaded target %s for %s", target, login)
}

// setStalkWatch installs (or replaces) the in-memory watch for a channel.
// Changing the target clears any last message from the previous chatter.
func (b *Bot) setStalkWatch(channel, target string) {
	channel = strings.ToLower(strings.TrimSpace(channel))
	target = strings.ToLower(strings.TrimSpace(target))
	if channel == "" || target == "" {
		return
	}

	b.stalkMu.Lock()
	defer b.stalkMu.Unlock()
	if existing, ok := b.stalk[channel]; ok && existing.target == target {
		return
	}
	b.stalk[channel] = &stalkWatch{target: target}
}

// hydrateStalkLast fills last from the DB only when memory has no line yet, so
// a restart can quote the persisted message without clobbering a newer in-memory
// line if load re-fires.
func (b *Bot) hydrateStalkLast(channel, last string) {
	last = strings.TrimSpace(last)
	if last == "" {
		return
	}
	channel = strings.ToLower(strings.TrimSpace(channel))

	b.stalkMu.Lock()
	defer b.stalkMu.Unlock()
	w, ok := b.stalk[channel]
	if !ok || w == nil || w.last != "" {
		return
	}
	w.last = last
}

// dropStalkTarget evicts a channel's !stalk watch. Called on stream.offline
// and on !stalk clear so an offline/unconfigured channel stops recording.
func (b *Bot) dropStalkTarget(login string) {
	key := strings.ToLower(login)
	b.stalkMu.Lock()
	delete(b.stalk, key)
	b.stalkMu.Unlock()
}

// stalkWatchFor returns the in-memory target and last message for a channel.
// ok is false when this channel has no watch (not configured, or not live).
func (b *Bot) stalkWatchFor(channel string) (target, last string, ok bool) {
	b.stalkMu.Lock()
	defer b.stalkMu.Unlock()
	w, ok := b.stalk[strings.ToLower(channel)]
	if !ok || w == nil {
		return "", "", false
	}
	return w.target, w.last, true
}

// rememberStalkMessage records text only when this channel is watching this
// chatter (login or display name matches the configured target). Everyone else
// is ignored — no per-chatter history is kept.
func (b *Bot) rememberStalkMessage(channel, login, display, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if isStalkCommand(strings.ToLower(text)) {
		return
	}

	channel = strings.ToLower(strings.TrimSpace(channel))
	login = strings.ToLower(strings.TrimSpace(login))
	display = strings.ToLower(strings.TrimSpace(display))
	if channel == "" || login == "" {
		return
	}
	if strings.EqualFold(login, b.config.BotUsername) {
		return
	}

	b.stalkMu.Lock()
	w, ok := b.stalk[channel]
	if !ok || w == nil {
		b.stalkMu.Unlock()
		return
	}
	if login != w.target && display != w.target {
		b.stalkMu.Unlock()
		return
	}
	if w.last == text {
		b.stalkMu.Unlock()
		return
	}
	w.last = text
	watchTarget := w.target
	b.stalkMu.Unlock()

	go b.persistStalkLast(channel, watchTarget, text)
}

func (b *Bot) persistStalkLast(channel, target, text string) {
	if b.coreAPI == nil {
		return
	}
	if err := b.coreAPI.SetStalkLastMessage(channel, target, text); err != nil {
		log.Printf("[STALK] [%s] persist last message failed: %v", channel, err)
	}
}
