package twitchbot

import "github.com/gempir/go-twitch-irc/v4"

func (b *Bot) handleJoinCommand(message *twitch.PrivateMessage) {
	if message.Channel != botCreatorChannel {
		return
	}

}
