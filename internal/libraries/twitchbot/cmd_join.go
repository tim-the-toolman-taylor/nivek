package twitchbot

import (
	"context"
	"github.com/gempir/go-twitch-irc/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
	"log"
)

func (b *Bot) handleJoinCommand(message *twitch.PrivateMessage) {
	// A home-channel owner (creator / the bot itself) joining themselves is a
	// no-op — they're already permanent.
	if b.isPermanentChannel(message.User.Name) {
		return
	}
	// Home channels (creator's + the bot's own) allow a bare !joinme; any other
	// channel requires an explicit @mention of the bot so it only acts on a
	// deliberate request.
	if !b.isPermanentChannel(message.Channel) && !mentionsBot(message.Message, b.config.BotUsername) {
		return
	}

	// The rest is network-bound (profile fetch, live check, persist, subscribe),
	// so run it off the IRC parser goroutine — a slow !joinme must not stall
	// message handling. All config.Channels access is guarded by channelsMu.
	go b.joinNewUser(message)
}

func (b *Bot) joinNewUser(message *twitch.PrivateMessage) {
	// Claim the join under the lock: skip if we already track this user, or if a
	// concurrent !joinme for them is already in flight (a spammed command must not
	// double-create the row or double-append the channel).
	// @TODO::convert channel slice back to a hash map - use broadcaster-id as a key
	b.channelsMu.Lock()
	for _, u := range b.config.Channels {
		if u.TwitchID != nil && *u.TwitchID == message.User.ID {
			b.channelsMu.Unlock()
			return
		}
	}
	if b.joinInFlight[message.User.ID] {
		b.channelsMu.Unlock()
		return
	}
	b.joinInFlight[message.User.ID] = true
	b.channelsMu.Unlock()

	defer func() {
		b.channelsMu.Lock()
		delete(b.joinInFlight, message.User.ID)
		b.channelsMu.Unlock()
	}()

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

	newUser := user.User{
		TwitchID:          &twitchUser.ID,
		TwitchLogin:       &twitchUser.Login,
		TwitchDisplayName: &twitchUser.DisplayName,
		BotOptIn:          true,
		IsLive:            isLive,
	}

	// add user to db
	if err := b.coreAPI.CreateNewUser(&newUser); err != nil {
		log.Printf(
			"failed to persist new user via !joinme %s (%s) - %s",
			message.User.Name,
			message.User.ID,
			err.Error(),
		)
		return
	}

	// subscribe to user webhooks (log failures so a silent subscription drop is visible)
	if _, subErr := b.twitchClient.SubscribeStreamOffline(context.Background(), *newUser.TwitchID); subErr != nil {
		log.Printf("failed to subscribe stream.offline for !joinme user %s: %s", message.User.Name, subErr.Error())
	}
	if _, subErr := b.twitchClient.SubscribeStreamOnline(context.Background(), *newUser.TwitchID); subErr != nil {
		log.Printf("failed to subscribe stream.online for !joinme user %s: %s", message.User.Name, subErr.Error())
	}

	// Track the new channel in-memory so the bot tracks it without a restart and a
	// repeat !joinme is recognized as a duplicate.
	b.channelsMu.Lock()
	// hasPrivs defaults to false: a freshly-joined channel hasn't modded the bot
	// yet, so the next command there triggers the "mod me" nudge until it does.
	b.config.Channels = append(b.config.Channels, BotUser{User: newUser})
	b.channelsMu.Unlock()
}
