package twitchbot

import (
	"context"
	"github.com/gempir/go-twitch-irc/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
	"log"
)

func (b *Bot) handleJoinCommand(message *twitch.PrivateMessage) {
	if message.Channel != botCreatorChannel || message.User.Name == botCreatorChannel {
		return
	}

	// check to see if we are already tracking this user
	// if we do, assume the command is just noise and disregard
	// @TODO::convert channel slice back to a hash map - use broadcaster-id as a key
	for _, u := range b.config.Channels {
		if *u.TwitchID == message.User.ID {
			return
		}
	}

	// we only need broadcast_user_id to subscribe to a new users's webhooks,
	// and their twitch_login to join their channel -- both of which are present
	// on their chat messages

	// the caveat comes when trying to fetch their profile from https://api.twitch.tv/helix/users/
	// this request requires a "code" which is only provided when the user starts the oauth flow

	// ATTEMPT to join channel

	// while joining simply `message.User.Name` will likely work - it is lossy
	// github.com/gempir/go-twitch-irc/v4/message.go:172-174 contains a fallback in case this value
	// is empty. The fallback fetches DisplayName and attempts to strings.ToLower and remove whitespace
	// client.Join does not return an error if it fails to join the channel, so I have no way of accounting
	// for this.
	b.client.Join(message.User.Name)

	twitchUser, err := b.twitchClient.FetchTwitchProfile(
		context.Background(),
		&message.User.ID,
		nil,
	)
	if err != nil {
		log.Printf(
			"failed to fetch user twitch profile for !join (%s) - %s",
			message.User.Name,
			err.Error(),
		)
		return
	}
	isLive, err := b.twitchClient.IsStreamLive(context.Background(), message.User.ID)
	if err != nil {
		log.Printf(
			"failed to determine if user is live during !join %s (%s) - %s",
			message.User.Name,
			message.User.ID,
			err.Error(),
		)
	}

	var user user.User
	user.TwitchID = &twitchUser.ID
	user.TwitchLogin = &twitchUser.Login
	user.TwitchDisplayName = &twitchUser.DisplayName
	user.BotOptIn = true
	user.IsLive = isLive

	// add user to db
	b.coreAPI.CreateNewUser(&user)

	// subscribe to user webhooks
	b.twitchClient.SubscribeStreamOffline(context.Background(), *user.TwitchID)
	b.twitchClient.SubscribeStreamOnline(context.Background(), *user.TwitchID)
}
