package twitchbot

import "log"

func (b *Bot) fetchAutoShoutChatters(broadcasterId, twitchLogin *string) {
	log.Printf("[Auto-Shout] Fetching chatters for %s!", *broadcasterId)

	chatters, err := b.coreAPI.GetAutoShoutChattersForChannel(broadcasterId)
	if err != nil {
		log.Printf("failed to fetch AutoShout chatters for channel %s - %s", *broadcasterId, err.Error())
	}

	log.Printf("[Auto-Shout] Chatters found: %s", chatters)

	b.autoShout[*twitchLogin] = chatters
}
