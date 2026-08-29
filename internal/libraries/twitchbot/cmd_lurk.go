package twitchbot

import "fmt"

func (b *Bot) handleLurkCommand(message *chatMessageEvent) {
	username := message.ChatterUserLogin
	channel := message.BroadcasterUserLogin

	if count := b.coreAPI.LurkOnMessage(channel, username); count > 0 {
		b.say(message.BroadcasterUserId, fmt.Sprintf(
			"thank you for the lurk! @%s You have lurked %d times",
			username,
			count,
		))
	}
}
