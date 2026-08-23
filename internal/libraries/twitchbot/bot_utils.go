package twitchbot

import (
	"context"
	"log"
	"strings"
)

func (b *Bot) isLegacyChannel(message *chatMessageEvent) {
	login := message.BroadcasterUserLogin

	// Claim the heal under the lock: find a still-legacy row for this sender that
	// isn't already being healed, and snapshot it. Mutating a range-copy here
	// would be a no-op (it wouldn't touch config.Channels), so we index in.
	b.channelsMu.Lock()
	idx := -1
	for i := range b.config.Channels {
		if strings.ToLower(b.config.Channels[i].Username) == login && b.config.Channels[i].TwitchID == nil {
			idx = i
			break
		}
	}
	if idx == -1 || b.healInFlight[login] {
		b.channelsMu.Unlock()
		return // not a pending legacy user, or a heal is already running for them
	}
	b.healInFlight[login] = true
	healed := b.config.Channels[idx] // snapshot under lock
	b.channelsMu.Unlock()

	healed.TwitchID = &message.BroadcasterUserId
	healed.TwitchDisplayName = &message.BroadcasterUserName
	healed.TwitchLogin = &message.BroadcasterUserLogin

	err := b.coreAPI.HealLegacyUser(&healed)
	if err != nil {
		log.Printf("failed to heal legacy user record: %+v - %s", healed, err.Error())
	} else {
		if _, subErr := b.twitchClient.SubscribeStreamOffline(context.Background(), *healed.TwitchID); subErr != nil {
			log.Printf("failed to subscribe stream.offline for healed user %s: %s", login, subErr.Error())
		}
		if _, subErr := b.twitchClient.SubscribeStreamOnline(context.Background(), *healed.TwitchID); subErr != nil {
			log.Printf("failed to subscribe stream.online for healed user %s: %s", login, subErr.Error())
		}
	}

	// Release the claim. On success, persist the patch into the in-memory list so
	// this user is never healed again this process; on failure, leave the row
	// legacy so a later message retries.
	b.channelsMu.Lock()
	delete(b.healInFlight, login)
	if err == nil && idx < len(b.config.Channels) &&
		strings.ToLower(b.config.Channels[idx].Username) == login {
		b.config.Channels[idx].TwitchID = healed.TwitchID
		b.config.Channels[idx].TwitchDisplayName = healed.TwitchDisplayName
		b.config.Channels[idx].TwitchLogin = healed.TwitchLogin
	}
	b.channelsMu.Unlock()
}

// isPermanentChannel reports whether the bot must always remain in the given
// channel (case-insensitive login match): the creator's channel and the bot's
// own channel. These are joined at boot regardless of live state, never departed
// on go-offline, and can never be banished.
func (b *Bot) isPermanentChannel(login string) bool {
	login = strings.ToLower(login)
	return login == botCreatorChannel || login == strings.ToLower(b.config.BotUsername)
}

// isTrackedChannel reports whether login is currently tracked in config.Channels
// (an opted-in or !joinme'd channel). Used to gate go-live joins so a banished
// (opted-out, removed) channel is not rejoined when its lingering stream.online
// subscription fires.
func (b *Bot) isTrackedChannel(login string) bool {
	login = strings.ToLower(login)
	b.channelsMu.Lock()
	defer b.channelsMu.Unlock()
	for i := range b.config.Channels {
		c := b.config.Channels[i]
		if c.TwitchLogin != nil && strings.ToLower(*c.TwitchLogin) == login {
			return true
		}
		if strings.ToLower(c.Username) == login {
			return true
		}
	}
	return false
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
