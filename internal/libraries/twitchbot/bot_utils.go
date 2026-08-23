package twitchbot

import "strings"

// isPermanentChannel reports whether the bot must always remain in the given
// channel (case-insensitive login match): the creator's channel and the bot's
// own channel. These are joined at boot regardless of live state, never departed
// on go-offline, and can never be banished.
func (b *Bot) isPermanentChannel(login string) bool {
	login = strings.ToLower(login)
	return login == botCreatorChannel || login == strings.ToLower(b.config.BotUsername)
}

// setLive records whether a channel (by lowercased login) is currently
// broadcasting. Called from the stream.online/offline webhooks and at boot.
func (b *Bot) setLive(login string, live bool) {
	login = strings.ToLower(login)
	b.liveMu.Lock()
	b.live[login] = live
	b.liveMu.Unlock()
}

// isChannelLive reports whether the bot should treat a channel as broadcasting
// for promo purposes. Permanent home channels always count as live so their
// promos (e.g. the bot's own self-promo) post regardless of stream state,
// preserving the pre-DB behavior.
func (b *Bot) isChannelLive(login string) bool {
	login = strings.ToLower(login)
	if b.isPermanentChannel(login) {
		return true
	}
	b.liveMu.Lock()
	defer b.liveMu.Unlock()
	return b.live[login]
}

// mentionsBot reports whether the message text @-mentions the bot by username.
// @TODO::refactor to use chatMessageEvent.Reply. This method can probably be removed if that property works as expected
func mentionsBot(message, botUsername string) bool {
	return strings.Contains(strings.ToLower(message), "@"+strings.ToLower(botUsername))
}

// isModOrBroadcaster reports whether the sender may run mod/broadcaster-gated
// commands in the channel the message came from.
// @TODO::hoist this earlier in the handle-message processing. It doesn't need to be handled individually everywhere, resolve it once and pass the resolution around
func isModOrBroadcaster(broadcasterUserId, chatterUserId string, badges []badges) bool {
	if broadcasterUserId == chatterUserId {
		return true
	}

	for _, badge := range badges {
		if badge.SetId == "moderator" {
			return true
		}
	}

	return false
}

func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// abs returns |n|. Used by dfSuccessReply when formatting a Mine
// action's dimensions for chat — Region.Max can be either >= or < Min
// depending on which corner the chatter typed first. Duplicated rather
// than imported because the overseer package's copy is unexported and
// the helper is too small to justify exposing or sharing.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
