package twitchbot

import (
	"fmt"
	"github.com/gempir/go-twitch-irc/v4"
)

func (b *Bot) handleLurkCommand(message *twitch.PrivateMessage) {
	username := message.User.Name
	channel := message.Channel

	if count := b.coreAPI.LurkOnMessage(channel, username); count > 0 {
		b.say(channel, fmt.Sprintf(
			"thank you for the lurk! @%s You have lurked %d times",
			username,
			count,
		))
	}
}
