package twitchbot

import (
	"context"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
	"log"
)

func (b *Bot) handleJoinCommand(message *chatMessageEvent) {
	// A home-channel owner (creator / the bot itself) joining themselves is a
	// no-op — they're already permanent.
	if b.isPermanentChannel(message.BroadcasterUserLogin) {
		return
	}
	// Home channels (creator's + the bot's own) allow a bare !joinme; any other
	// channel requires an explicit @mention of the bot so it only acts on a
	// deliberate request.
	if !mentionsBot(message.Message.Text, b.config.BotUsername) {
		return
	}

	// The rest is network-bound (profile fetch, live check, persist, subscribe),
	// so run it off the IRC parser goroutine — a slow !joinme must not stall
	// message handling. All config.Channels access is guarded by channelsMu.
	go b.joinNewUser(message)
}

func (b *Bot) joinNewUser(message *chatMessageEvent) {
	// Claim the join under the lock: skip if we already track this user, or if a
	// concurrent !joinme for them is already in flight (a spammed command must not
	// double-create the row or double-append the channel).
	// @TODO::convert channel slice back to a hash map - use broadcaster-id as a key
	b.channelsMu.Lock()
	for _, u := range b.config.Channels {
		if u.TwitchID != nil && *u.TwitchID == message.ChatterUserId {
			b.channelsMu.Unlock()
			return
		}
	}
	if b.joinInFlight[message.ChatterUserId] {
		b.channelsMu.Unlock()
		return
	}
	b.joinInFlight[message.ChatterUserId] = true
	b.channelsMu.Unlock()

	defer func() {
		b.channelsMu.Lock()
		delete(b.joinInFlight, message.ChatterUserId)
		b.channelsMu.Unlock()
	}()

	twitchUser, err := b.twitchClient.FetchTwitchProfile(
		context.Background(),
		&message.ChatterUserId,
		nil,
	)
	if err != nil {
		log.Printf(
			"failed to fetch user twitch profile for !join (%s) - %s",
			message.ChatterUserId,
			err.Error(),
		)
		return
	}
	// @TODO::once I go-live with dropping the IRC approach, review if the "isLive" state-management system is needed
	isLive, err := b.twitchClient.IsStreamLive(context.Background(), message.ChatterUserId)
	if err != nil {
		log.Printf(
			"failed to determine if user is live during !join %s (%s) - %s",
			message.ChatterUserId,
			message.ChatterUserId,
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
			message.ChatterUserLogin,
			message.ChatterUserId,
			err.Error(),
		)
		return
	}

	// subscribe to user webhooks (log failures so a silent subscription drop is visible)
	if _, subErr := b.twitchClient.SubscribeStreamOffline(context.Background(), *newUser.TwitchID); subErr != nil {
		log.Printf("failed to subscribe stream.offline for !joinme user %s: %s", message.ChatterUserLogin, subErr.Error())
	}
	if _, subErr := b.twitchClient.SubscribeStreamOnline(context.Background(), *newUser.TwitchID); subErr != nil {
		log.Printf("failed to subscribe stream.online for !joinme user %s: %s", message.ChatterUserLogin, subErr.Error())
	}
	if _, subErr := b.twitchClient.SubscribeChannelChatMessages(context.Background(), *newUser.TwitchID); subErr != nil {
		log.Printf("failed to subscribe to channel.chat.message for !joinme user %s: %s", message.ChatterUserLogin, subErr.Error())
	}

	// Track the new channel in-memory so the bot tracks it without a restart and a
	// repeat !joinme is recognized as a duplicate.
	b.channelsMu.Lock()
	b.config.Channels = append(b.config.Channels, newUser)
	b.channelsMu.Unlock()
}
