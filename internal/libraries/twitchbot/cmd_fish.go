package twitchbot

import (
	"github.com/gempir/go-twitch-irc/v4"
	"log"
)

func (b *Bot) handleFishCommand(message *twitch.PrivateMessage) {
	username := message.User.Name
	channel := message.Channel

	response, err := b.coreAPI.GoFishing(channel, username)
	if err != nil {
		log.Printf("error running fish for channel [%s] chatter [%s]: %s", channel, username, err.Error())
		return
	}
	b.say(channel, response)
	log.Printf("[FISH] [%s] %s", channel, username)
}
