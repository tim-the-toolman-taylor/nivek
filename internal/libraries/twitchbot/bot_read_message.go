package twitchbot

import (
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"
)

type chatMessageEvent struct {
	BroadcasterUserId    string          `json:"broadcaster_user_id"`
	BroadcasterUserLogin string          `json:"broadcaster_user_login"`
	BroadcasterUserName  string          `json:"broadcaster_user_name"`
	ChatterUserId        string          `json:"chatter_user_id"`
	ChatterUserLogin     string          `json:"chatter_user_login"`
	ChatterUserName      string          `json:"chatter_user_name"`
	MessageId            string          `json:"message_id"`
	Message              chatMessageBody `json:"message"`
	Color                string          `json:"color"`
	Badges               []badges        `json:"badges"`
	// Badges example content:
	// [
	//   {
	//     "set_id": "moderator",
	//     "id": "1",
	//     "info": ""
	//   },
	//   {
	//     "set_id": "subscriber",
	//     "id": "12",
	//     "info": "16"
	//   },
	//   {
	//     "set_id": "sub-gifter",
	//     "id": "1",
	//     "info": ""
	//   }
	// ],
	MessageType                 string  `json:"message_type"`
	Cheer                       *string `json:"cheer,omitempty"`
	Reply                       *string `json:"reply,omitempty"`
	ChannelPointsCustomRewardId *string `json:"channel_points_custom_reward_id,omitempty"`
	SourceBroadcasterUserId     *string `json:"source_broadcaster_user_id,omitempty"`
	SourceBroadcasterUserLogin  *string `json:"source_broadcaster_user_login,omitempty"`
	SourceBroadcasterUserName   *string `json:"source_broadcaster_user_name,omitempty"`
	SourceMessageId             *string `json:"source_message_id,omitempty"`
	SourceBadges                *string `json:"source_badges,omitempty"`

	// matchedTrigger is the command word the dispatcher matched, set just before
	// a handler runs. Not part of the Twitch payload -- it exists so a handler
	// can find its own arguments in a message that may contain several commands.
	matchedTrigger string
}

type badges struct {
	SetId string `json:"set_id"`
	Id    string `json:"id"`
	Info  string `json:"info"`
}

type chatMessageBody struct {
	Text      string     `json:"text"`
	Fragments []fragment `json:"fragments"`
	// example Fragments content:
	// [
	//        {
	//          "type": "text",
	//          "text": "Hi chat",
	//          "cheermote": null,
	//          "emote": null,
	//          "mention": null
	// 	}
	// ]
}

type fragment struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Cheermote *any   `json:"cheermote,omitempty"`
	Emote     *any   `json:"emote,omitempty"`
	Mention   *any   `json:"mention,omitempty"`
}

func (b *Bot) handleWebhookMessage(notification *EventSubSubscriptionResponse) {
	var messageEvent chatMessageEvent
	if err := json.Unmarshal(notification.Event, &messageEvent); err != nil {
		log.Printf("failed to read chat message Event: %s", err.Error())
		return
	}

	if b.seenChatIDs != nil && b.seenChatIDs.seen(messageEvent.MessageId) {
		log.Printf("[EVENTSUB] duplicate chat message_id=%s; dropping", messageEvent.MessageId)
		return
	}

	if messageEvent.ChatterUserId == b.config.BotId || strings.EqualFold(messageEvent.ChatterUserLogin, b.config.BotUsername) {
		return
	}

	b.rememberStalkMessage(
		messageEvent.BroadcasterUserLogin,
		messageEvent.ChatterUserLogin,
		messageEvent.ChatterUserName,
		messageEvent.Message.Text,
	)

	msg := strings.ToLower(messageEvent.Message.Text)
	channel := messageEvent.BroadcasterUserLogin
	channelId := messageEvent.BroadcasterUserId
	chatter := messageEvent.ChatterUserLogin

	// !newpromo takes free-form arguments (an interval + a message that may itself
	// contain other command words or URLs). Dispatch it up front and return so the
	// word-by-word scan below never treats the promo body as more commands.
	if strings.HasPrefix(msg, "!newpromo ") {
		log.Printf("[CMD-RECV] [%s] %s: %q", channel, chatter, msg)
		b.handleNewPromoCommand(&messageEvent)
		return
	}

	// !stalk also takes arguments (`set` / `clear` / a username). Dispatch it up
	// front so a target named after another command isn't also fired, and so the
	// no-arg form still works if the DB seed hasn't loaded into b.commands yet.
	if isStalkCommand(msg) {
		log.Printf("[CMD-RECV] [%s] %s: %q", channel, chatter, msg)
		b.handleStalkCommand(&messageEvent)
		return
	}

	// Check for commands
	for msgword := range strings.SplitSeq(msg, " ") {
		// A capability-gated global (nivek.command.requires) only dispatches in
		// channels holding that capability. When it does not, we deliberately do
		// NOT skip the word: we fall through to the channel's own commands
		// below. Skipping would let a global row claim a trigger everywhere and
		// then decline to answer it, which is worse than not registering it —
		// see docs/OVERLAY_COMMANDS.md, "The fall-through".
		if handler, ok := b.commands[msgword]; ok && b.commandAllowedIn(channel, msgword) {
			log.Printf("[CMD-RECV] [%s] %s: %q", channel, chatter, msg)
			messageEvent.matchedTrigger = msgword
			handler(b, &messageEvent)
			continue
		}
		// Per-channel custom commands (loaded while the channel is live). An
		// ungated global with the same trigger wins — handled above via continue
		// — so a channel can't shadow a builtin in v1.
		if cmd, ok := b.customCommandFor(channel, msgword); ok {
			if cmd.ResponseTmpl != nil && meetsMinRole(&messageEvent, cmd.MinRole) {
				log.Printf("[CMD-RECV] [%s] %s: %q (custom)", channel, chatter, msg)
				b.say(channelId, *cmd.ResponseTmpl)
			}
		}
	}

	if _, ok := b.autoShout[channel]; ok {
		if slices.Contains(
			b.autoShout[channel],
			messageEvent.ChatterUserName,
		) {
			b.say(channelId, fmt.Sprintf("!so @%s", chatter))
			log.Printf("[Auto Shout] given to %s in %s", chatter, channel)
			// Persist the shout: bump shout_count and stamp this stream's key so
			// a restart mid-stream won't re-shout them. Off the message path.
			go b.incrementAutoShout(channel, messageEvent.ChatterUserName)
			i := slices.Index(b.autoShout[channel], messageEvent.ChatterUserName)
			b.autoShout[channel] = slices.Delete(
				b.autoShout[channel],
				i,
				i+1,
			)
		}
	}

	// DF commands are suspended until further notice
	// !DF takes arguments — handle separately from the exact-match commands below
	// if msg == "!df" || strings.HasPrefix(msg, "!df ") {
	// 	if message.Channel != botCreatorChannel {
	// 		return
	// 	}
	// 	args := strings.TrimSpace(strings.TrimPrefix(msg, "!df"))
	// 	b.handleDFCommand(message.Message, args, chattername, message.Channel)
	// 	return
	// }
}
