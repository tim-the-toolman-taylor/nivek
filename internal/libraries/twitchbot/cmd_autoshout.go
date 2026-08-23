package twitchbot

import (
	"log"
	"strconv"
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
func (b *Bot) incrementAutoShout(broadcasterId, chattername string) {
	id, err := strconv.Atoi(broadcasterId)
	if err != nil {
		log.Printf("[Auto Shout] invalid twitch_id %q: %v", broadcasterId, err)
		return
	}
	if err := b.coreAPI.IncrementAutoShoutCount(id, chattername); err != nil {
		log.Printf("[Auto Shout] failed to increment shout count for %s: %v", chattername, err)
	}
}
