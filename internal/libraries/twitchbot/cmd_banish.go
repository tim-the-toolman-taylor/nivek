package twitchbot

import (
	"context"
	"log"
	"strings"

	"github.com/gempir/go-twitch-irc/v4"
)

func isBanishCommand(msg string) bool {
	return msg == "!banish" || strings.HasPrefix(msg, "!banish ")
}

// handleBanishCommand is the EventSub path for !banish.
func (b *Bot) handleBanishCommand(message *chatMessageEvent) {
	if !isModOrBroadcaster(message) {
		return
	}
	if b.isPermanentChannel(message.BroadcasterUserLogin) {
		b.say(message.BroadcasterUserId, "not leaving this one 😤")
		return
	}
	go b.banishChannel(message.BroadcasterUserLogin, message.BroadcasterUserId, true)
}

// handleIRCBanish is the IRC path for !banish (channels that have not granted
// chat-read yet). Replies over IRC because Helix send is rejected until the
// bot is modded / granted channel:bot.
func (b *Bot) handleIRCBanish(message twitch.PrivateMessage) {
	if !message.User.IsBroadcaster && !message.User.IsMod && message.User.ID != message.RoomID {
		return
	}
	if b.isPermanentChannel(message.Channel) {
		b.ircsay(message.Channel, "not leaving this one 😤")
		return
	}
	go b.banishChannel(message.Channel, message.RoomID, false)
}

func (b *Bot) banishChannel(channel, channelId string, viaHelix bool) {
	channel = strings.ToLower(channel)

	if viaHelix && channelId != "" {
		b.say(channelId, "aight, I'm out ✌️")
	} else {
		b.ircsay(channel, "aight, I'm out ✌️")
	}
	b.client.Depart(channel)
	if channelId != "" {
		b.unsubscribeChannelWebhooks(channelId)
	}

	b.setLive(channel, false)

	if err := b.coreAPI.OptOutUser(channel); err != nil {
		log.Printf("[BANISH] [%s] opt-out failed: %v", channel, err)
	}

	b.channelsMu.Lock()
	filtered := b.config.Channels[:0]
	for _, u := range b.config.Channels {
		if u.TwitchLogin == nil || *u.TwitchLogin != channel {
			filtered = append(filtered, u)
		}
	}
	b.config.Channels = filtered
	b.channelsMu.Unlock()

	log.Printf("[BANISH] [%s] departed, unsubscribed, and opted out", channel)
}

func (b *Bot) unsubscribeChannelWebhooks(channelId string) {
	ctx := context.Background()
	subs, err := b.twitchClient.ListEventSubSubscriptions(ctx)
	if err != nil {
		log.Printf("[BANISH] list eventsub for %s failed: %v", channelId, err)
		return
	}
	for _, s := range subs {
		if s.Condition.BroadcasterUserID != channelId {
			continue
		}
		if err := b.twitchClient.DeleteEventSubSubscription(ctx, s.ID); err != nil {
			log.Printf("[BANISH] delete eventsub %s (%s) for %s failed: %v", s.ID, s.Type, channelId, err)
		}
	}
}
