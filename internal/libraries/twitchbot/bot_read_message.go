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
}

type badges struct {
	SetId string `json:"set_id"`
	Id    string `json:"id"`
	Info  string `json:"info"`
}

type chatMessageBody struct {
	Text      string   `json:"text"`
	Fragments []string `json:"fragments"`
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

func (b *Bot) handleMessage(notification *EventSubSubscriptionResponse) error {
	var messageEvent chatMessageEvent
	if err := json.Unmarshal(notification.Event, &messageEvent); err != nil {
		return fmt.Errorf("failed to read chat message Event: %s", err.Error())
	}

	// Normalize message
	msg := strings.TrimSpace(strings.ToLower(messageEvent.Message.Text))

	// !newpromo takes free-form arguments (an interval + a message that may itself
	// contain other command words or URLs). Dispatch it up front and return so the
	// word-by-word scan below never treats the promo body as more commands.
	if msg == "!newpromo" || strings.HasPrefix(msg, "!newpromo ") {
		log.Printf("[CMD-RECV] [%s] %s: %q",
			messageEvent.BroadcasterUserLogin,
			messageEvent.ChatterUserLogin,
			msg,
		)
		b.handleNewPromoCommand(&messageEvent)
		return nil
	}

	// Check for commands
	for msgword := range strings.SplitSeq(msg, " ") {
		if handler, ok := b.commands[msgword]; ok {
			log.Printf("[CMD-RECV] [%s] %s: %q", messageEvent.BroadcasterUserLogin, messageEvent.ChatterUserLogin, msg)
			handler(b, &messageEvent)
		}
	}

	if _, ok := b.autoShout[messageEvent.BroadcasterUserLogin]; ok {
		if slices.Contains(
			b.autoShout[messageEvent.BroadcasterUserLogin],
			messageEvent.ChatterUserLogin,
		) {
			// @TODO::make auto shout actually perform the auto shout instead of triggering a moobot command XD
			resp := fmt.Sprintf("!so @%s", messageEvent.ChatterUserLogin)
			b.say(
				&messageEvent.BroadcasterUserId,
				&resp,
			)
			log.Printf("[Auto Shout] given to %s in %s", messageEvent.ChatterUserLogin, messageEvent.BroadcasterUserLogin)
			// Persist the shout: bump shout_count and stamp this stream's key so
			// a restart mid-stream won't re-shout them. Off the message path.
			go b.incrementAutoShout(messageEvent.BroadcasterUserLogin, messageEvent.ChatterUserLogin)
			i := slices.Index(b.autoShout[messageEvent.BroadcasterUserLogin], messageEvent.ChatterUserLogin)
			b.autoShout[messageEvent.BroadcasterUserLogin] = slices.Delete(
				b.autoShout[messageEvent.BroadcasterUserLogin],
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

	return nil
}
