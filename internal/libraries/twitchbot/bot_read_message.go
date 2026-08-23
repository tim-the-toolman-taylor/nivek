package twitchbot

import (
	"encoding/json"
	"fmt"
	"log"
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
	Type      string  `json:"type"`
	Text      string  `json:"text"`
	Cheermote *string `json:"cheermote,omitempty"`
	Emote     *any    `json:"emote,omitempty"`
	Mention   *any    `json:"mention,omitempty"`
}

func (b *Bot) handleWebhookMessage(notification *EventSubSubscriptionResponse) error {
	var messageEvent chatMessageEvent
	if err := json.Unmarshal(notification.Event, &messageEvent); err != nil {
		return fmt.Errorf("failed to read chat message Event: %s", err.Error())
	}

	log.Printf("[CHANNEL CHAT MESSAGE] Chat message recieved from webhook! - %s", messageEvent.Message.Text)

	return nil
}
