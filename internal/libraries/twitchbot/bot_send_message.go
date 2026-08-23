package twitchbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// @TODO::investigate if send-logic can be moved to a different file as this one is getting longer than I like
func (b *Bot) sayChatMessage(broadcasterId, message string) error {
	type sendChatMessageBody struct {
		BroadcasterId string `json:"broadcaster_id"`
		SenderId      string `json:"sender_id"`
		Message       string `json:"message"`
		ForSourceOnly *bool  `json:"for_source_only,omitempty"`
	}

	chatMessageBod := sendChatMessageBody{
		BroadcasterId: broadcasterId,
		SenderId:      b.config.BotId,
		Message:       message,
	}

	chatMessageBodByte, err := json.Marshal(chatMessageBod)
	if err != nil {
		return fmt.Errorf("failed to convert message body to []byte: %s", err.Error())
	}

	var bodyReader io.Reader
	bodyReader = bytes.NewReader(chatMessageBodByte)
	req, err := http.NewRequest(http.MethodPost, "https://api.twitch.tv/helix/chat/messages", bodyReader) // @TODO::move hardcoded URL to a const
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Client-Id", b.config.BotOAuth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("failed to send chat message: status %d - %s", resp.StatusCode, respBody)
	}

	// https://dev.twitch.tv/docs/api/reference/#send-chat-message
	type sendChatMessageResponse struct {
		Data       interface{} `json:"data"`
		MessageId  string      `json:"message_id"`  // the message id for the message that was sent
		IsSent     bool        `json:"is_sent"`     // if the message passed all checks and was sent
		DropReason interface{} `json:"drop_reason"` // the reason the message was dropped, if any
		Code       string      `json:"code"`        // code for why the message was dropped
		Message    string      `json:"message"`     // message for why the message was dropped
	}

	var decodedResp sendChatMessageResponse
	if err := json.Unmarshal(respBody, &decodedResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	// @TODO::more thourough error handling and logging verbosity
	if !decodedResp.IsSent {
		return fmt.Errorf("message failed to send: [%s] %s", decodedResp.Code, decodedResp.Message)
	}

	return nil
}

func (b *Bot) senderLoop() {
	for req := range b.sayQueue {
		b.sayChatMessage(req.broadcasterId, req.message)
		time.Sleep(1500 * time.Millisecond)
	}
}

func (b *Bot) say(broadcasterId, message *string) {
	b.sayQueue <- sayRequest{*broadcasterId, *message}
}
