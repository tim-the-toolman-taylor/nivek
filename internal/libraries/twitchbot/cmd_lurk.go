package twitchbot

import "fmt"

func (b *Bot) handleLurkCommand(username, channel string) {
	if count := b.coreAPI.LurkOnMessage(channel, username); count > 0 {
		b.say(channel, fmt.Sprintf(
			"thank you for the lurk! @%s You have lurked %d times",
			username,
			count,
		))
	}
}
