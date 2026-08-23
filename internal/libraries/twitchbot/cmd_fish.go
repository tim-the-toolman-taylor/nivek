package twitchbot

import (
	"log"
)

func (b *Bot) handleFishCommand(message *chatMessageEvent) {
	username := message.ChatterUserLogin
	channel := message.BroadcasterUserLogin

	response, err := b.coreAPI.GoFishing(channel, username)
	if err != nil {
		log.Printf("error running fish for channel [%s] chatter [%s]: %s", channel, username, err.Error())
		return
	}
	b.say(&message.BroadcasterUserId, &response)
	log.Printf("[FISH] [%s] %s", channel, username)
}
