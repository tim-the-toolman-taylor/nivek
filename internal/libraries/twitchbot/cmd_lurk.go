package twitchbot

import "fmt"

func (b *Bot) handleLurkCommand(message *chatMessageEvent) {
	username := message.ChatterUserLogin
	channel := message.BroadcasterUserLogin

	if count := b.coreAPI.LurkOnMessage(channel, username); count > 0 {
		resp := fmt.Sprintf(
			"thank you for the lurk! @%s You have lurked %d times",
			username,
			count,
		)
		b.say(&message.BroadcasterUserId, &resp)
	}
}
