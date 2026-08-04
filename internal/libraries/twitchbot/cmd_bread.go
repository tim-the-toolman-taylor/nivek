package twitchbot

import (
	"fmt"
	"github.com/gempir/go-twitch-irc/v4"
	"log"
)

func (b *Bot) handleBreadCommand(message *twitch.PrivateMessage) {
	username := message.User.Name
	channel := message.Channel

	count, err := b.coreAPI.IncrementBread(channel, username)
	if err != nil {
		log.Printf("error incrementing bread count for channel [%s] chatter [%s]: %s", channel, username, err.Error())
		return
	}
	totalCount, err := b.coreAPI.GetBreadTotal(channel)
	if err != nil {
		log.Printf("error getting total bread count for channel [%s] chatter [%s]: %s", channel, username, err.Error())
		return
	}

	response := fmt.Sprintf(
		"@%s has baked %d loaf%s of bread today! 🍞 This chat has baked %d loaf%s total in the last 24 hours.",
		username,
		count,
		pluralize(count),
		totalCount,
		pluralize(totalCount),
	)

	b.say(channel, response)
	log.Printf("[BREAD] [%s] %s: %d (Total: %d)", channel, username, count, totalCount)
}
