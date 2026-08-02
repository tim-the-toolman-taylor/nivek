package twitchbot

import "log"

func (b *Bot) handleFishCommand(username, channel string) {
	response, err := b.coreAPI.GoFishing(channel, username)
	if err != nil {
		log.Printf("error running fish for channel [%s] chatter [%s]: %s", channel, username, err.Error())
		return
	}
	b.say(channel, response)
	log.Printf("[FISH] [%s] %s", channel, username)
}
