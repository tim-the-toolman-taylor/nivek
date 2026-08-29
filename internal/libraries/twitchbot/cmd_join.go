package twitchbot

import (
	"context"
	"log"
	"strings"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

// handleJoinCommand implements !joinme. Dispatch is EventSub-only (the channel
// the command was typed in must already have chat-read), but the *target* is
// the chatter's own channel and is joined over IRC so the bot can ask to be
// modded there. Home channels allow a bare !joinme; anywhere else requires an
// explicit @mention of the bot.
func (b *Bot) handleJoinCommand(message *chatMessageEvent) {
	if b.isPermanentChannel(message.ChatterUserLogin) {
		return
	}
	if !b.isPermanentChannel(message.BroadcasterUserLogin) && !mentionsBot(message.Message.Text, b.config.BotUsername) {
		return
	}

	go b.joinNewUser(message)
}

func (b *Bot) joinNewUser(message *chatMessageEvent) {
	chatterID := message.ChatterUserId
	chatterLogin := strings.ToLower(message.ChatterUserLogin)

	b.channelsMu.Lock()
	for _, u := range b.config.Channels {
		if u.TwitchID != nil && *u.TwitchID == chatterID {
			b.channelsMu.Unlock()
			return
		}
	}
	if b.joinInFlight[chatterID] {
		b.channelsMu.Unlock()
		return
	}
	b.joinInFlight[chatterID] = true
	b.channelsMu.Unlock()

	defer func() {
		b.channelsMu.Lock()
		delete(b.joinInFlight, chatterID)
		b.channelsMu.Unlock()
	}()

	// IRC join first: the target has not granted channel.chat.message yet, so
	// the only way to reach them is the legacy connection (the "mod me" nudge).
	b.client.Join(chatterLogin)

	twitchUser, err := b.twitchClient.FetchTwitchProfile(context.Background(), &chatterID, nil)
	if err != nil {
		log.Printf("failed to fetch user twitch profile for !joinme (%s) - %s", chatterLogin, err.Error())
		return
	}
	isLive, err := b.twitchClient.IsStreamLive(context.Background(), chatterID)
	if err != nil {
		log.Printf("failed to determine if user is live during !joinme %s (%s) - %s", chatterLogin, chatterID, err.Error())
	}

	newUser := user.User{
		TwitchID:          &twitchUser.ID,
		TwitchLogin:       &twitchUser.Login,
		TwitchDisplayName: &twitchUser.DisplayName,
		BotOptIn:          true,
		IsLive:            isLive,
	}

	if err := b.coreAPI.CreateNewUser(&newUser); err != nil {
		log.Printf("failed to persist new user via !joinme %s (%s) - %s", chatterLogin, chatterID, err.Error())
		return
	}

	if _, subErr := b.twitchClient.SubscribeStreamOffline(context.Background(), *newUser.TwitchID); subErr != nil {
		log.Printf("failed to subscribe stream.offline for !joinme user %s: %s", chatterLogin, subErr.Error())
	}
	if _, subErr := b.twitchClient.SubscribeStreamOnline(context.Background(), *newUser.TwitchID); subErr != nil {
		log.Printf("failed to subscribe stream.online for !joinme user %s: %s", chatterLogin, subErr.Error())
	}

	b.channelsMu.Lock()
	b.config.Channels = append(b.config.Channels, BotUser{User: newUser})
	b.channelsMu.Unlock()
}
