package twitchbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const helixSendChatMessageURL = "https://api.twitch.tv/helix/chat/messages"

func (b *Bot) helixAccessToken(ctx context.Context) (string, error) {
	if b.tokenProvider != nil {
		tok, err := b.tokenProvider(ctx)
		if err != nil {
			return "", err
		}
		if tok == "" {
			return "", fmt.Errorf("token provider returned an empty token")
		}
		return tok, nil
	}
	tok := strings.TrimPrefix(b.config.BotOAuth, "oauth:")
	if tok == "" {
		return "", fmt.Errorf("no bot user token")
	}
	return tok, nil
}

func (b *Bot) sayChatMessage(broadcasterId, message string) error {
	if b.config.ClientID == "" {
		return fmt.Errorf("missing Twitch client id")
	}
	if broadcasterId == "" {
		return fmt.Errorf("missing broadcaster id")
	}

	ctx := context.Background()
	token, err := b.helixAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("bot user token: %w", err)
	}

	type sendChatMessageBody struct {
		BroadcasterId string `json:"broadcaster_id"`
		SenderId      string `json:"sender_id"`
		Message       string `json:"message"`
		ForSourceOnly *bool  `json:"for_source_only,omitempty"`
	}

	body, err := json.Marshal(sendChatMessageBody{
		BroadcasterId: broadcasterId,
		SenderId:      b.config.BotId,
		Message:       message,
	})
	if err != nil {
		return fmt.Errorf("failed to convert message body to []byte: %s", err.Error())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, helixSendChatMessageURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Client-Id", b.config.ClientID)
	req.Header.Set("Authorization", "Bearer "+token)
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

	// Helix nests message_id / is_sent / drop_reason under data[0].
	var decodedResp struct {
		Data []struct {
			MessageId  string `json:"message_id"`
			IsSent     bool   `json:"is_sent"`
			DropReason *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"drop_reason"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &decodedResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if len(decodedResp.Data) == 0 {
		return fmt.Errorf("helix send-chat-message returned empty data")
	}
	sent := decodedResp.Data[0]
	if !sent.IsSent {
		code, msg := "", ""
		if sent.DropReason != nil {
			code, msg = sent.DropReason.Code, sent.DropReason.Message
		}
		return fmt.Errorf("message failed to send: [%s] %s", code, msg)
	}

	return nil
}

func (b *Bot) senderLoop() {
	for req := range b.sayQueue {
		if err := b.sayChatMessage(req.broadcasterId, req.message); err != nil {
			log.Printf("[SAY] send to %s failed: %v", req.broadcasterId, err)
		}
		time.Sleep(1500 * time.Millisecond)
	}
}

func (b *Bot) say(broadcasterId, message string) {
	b.sayQueue <- sayRequest{broadcasterId, message}
}
