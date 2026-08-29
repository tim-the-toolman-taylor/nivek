package twitchbot

import (
	"log"
	"strings"
)

// handleBanishCommand lets a broadcaster or mod permanently remove the bot from
// their channel. The bot departs, the user is opted out (bot_opt_in=false) so a
// reboot's GetActiveChannels won't rejoin them, and the channel is dropped from
// in-memory tracking so a lingering stream.online subscription won't rejoin it
// either (see isTrackedChannel + handleGoLive). The permanent home channels can
// never be banished.
func (b *Bot) handleBanishCommand(message *chatMessageEvent) {
	if !isModOrBroadcaster(message) {
		return
	}
	if b.isPermanentChannel(message.ChatterUserLogin) {
		b.say(message.BroadcasterUserId, "not leaving this one 😤")
		return
	}

	// Teardown is network-bound (core-api opt-out) — run it off the IRC parser
	// goroutine. config.Channels access is guarded by channelsMu.
	go b.banishChannel(message.BroadcasterUserLogin, message.BroadcasterUserId)
}

func (b *Bot) banishChannel(channel, channelId string) {
	channel = strings.ToLower(channel)

	// Say goodbye synchronously (direct, not via the async queue) so it actually
	// lands before we PART the channel, then depart.
	b.say(channelId, "aight, I'm out ✌️")
	// @TODO::unsub from webhooks

	// Stop treating the channel as live so the promo scheduler doesn't post into a
	// channel we've just left.
	b.setLive(channel, false)

	// Persist the opt-out so a reboot no longer returns this channel.
	if err := b.coreAPI.OptOutUser(channel); err != nil {
		log.Printf("[BANISH] [%s] opt-out failed: %v", channel, err)
	}

	// Drop from in-memory tracking so a future go-live webhook won't rejoin
	// (in-place filter under the lock).
	b.channelsMu.Lock()
	filtered := b.config.Channels[:0]
	for _, u := range b.config.Channels {
		if u.TwitchLogin != nil && *u.TwitchLogin == channel {
			filtered = append(filtered, u)
		}
	}
	b.config.Channels = filtered
	b.channelsMu.Unlock()

	log.Printf("[BANISH] [%s] departed and opted out", channel)
}
