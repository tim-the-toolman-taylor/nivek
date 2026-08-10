package twitchbot

import (
	"log"
	"strconv"
	"strings"
)

func (b *Bot) fetchAutoShoutChatters(broadcasterId, twitchLogin *string) {
	log.Printf("[Auto-Shout] Fetching chatters for %s!", *broadcasterId)

	chatters, err := b.coreAPI.GetAutoShoutChattersForChannel(broadcasterId)
	if err != nil {
		log.Printf("failed to fetch AutoShout chatters for channel %s - %s", *broadcasterId, err.Error())
	}

	log.Printf("[Auto-Shout] Chatters found: %s", chatters)

	b.autoShout[*twitchLogin] = chatters
}

// incrementAutoShout bumps a chatter's shout_count via core-api after the bot
// fires an auto-shoutout for them. Best-effort and meant to run in a goroutine:
// it resolves the broadcaster's Twitch id from the tracked channel list, then
// calls the increment; every failure is logged, never fatal (the shoutout has
// already gone out).
func (b *Bot) incrementAutoShout(channelLogin, chattername string) {
	tid, ok := b.channelTwitchID(channelLogin)
	if !ok {
		log.Printf("[Auto Shout] no twitch_id for channel %s; skipping shout count increment", channelLogin)
		return
	}
	id, err := strconv.Atoi(tid)
	if err != nil {
		log.Printf("[Auto Shout] invalid twitch_id %q for channel %s: %v", tid, channelLogin, err)
		return
	}
	if err := b.coreAPI.IncrementAutoShoutCount(id, chattername); err != nil {
		log.Printf("[Auto Shout] failed to increment shout count for %s in %s: %v", chattername, channelLogin, err)
	}
}

// channelTwitchID returns the broadcaster Twitch id for a channel login from the
// tracked channel list, matching case-insensitively on twitch_login.
func (b *Bot) channelTwitchID(login string) (string, bool) {
	b.channelsMu.Lock()
	defer b.channelsMu.Unlock()
	for _, ch := range b.config.Channels {
		if ch.TwitchLogin != nil && strings.EqualFold(*ch.TwitchLogin, login) && ch.TwitchID != nil && *ch.TwitchID != "" {
			return *ch.TwitchID, true
		}
	}
	return "", false
}
