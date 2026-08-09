package twitchbot

import "log"

func (b *Bot) fetchAutoShoutChatters(broadcasterId string) {
	chatters, err := b.coreAPI.GetAutoShoutChattersForChannel(broadcasterId)
	if err != nil {
		log.Printf("failed to fetch AutoShout chatters for channel %s - %s", broadcasterId, err.Error())
	}

	b.autoShout[broadcasterId] = chatters
}
