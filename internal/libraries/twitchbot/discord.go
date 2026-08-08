package twitchbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// discordWebhookTimeout bounds the go-live POST so a slow/unreachable Discord
// can never wedge the goroutine that fires it.
const discordWebhookTimeout = 10 * time.Second

// discordEmbed is the minimal slice of Discord's webhook embed schema we use.
// https://discord.com/developers/docs/resources/webhook#execute-webhook
type discordEmbed struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Color       int    `json:"color"`
}

type discordWebhookPayload struct {
	Content string         `json:"content,omitempty"`
	Embeds  []discordEmbed `json:"embeds,omitempty"`
}

// notifyDiscordGoLive posts a "channel is live" message to the Discord webhook
// named by TWITCH_BOT_DISCORD_WEBHOOK. It is a no-op when the URL is unset, so
// the feature is opt-in per environment. Network-bound — call it in a goroutine,
// never on the webhook/parser path.
func notifyDiscordGoLive(displayName, login string) {
	webhookURL := os.Getenv("TWITCH_BOT_DISCORD_WEBHOOK")
	if webhookURL == "" {
		return
	}

	// Fall back to the login if Twitch sent an empty display name.
	name := displayName
	if strings.TrimSpace(name) == "" {
		name = login
	}
	streamURL := fmt.Sprintf("https://twitch.tv/%s", login)

	payload := discordWebhookPayload{
		Embeds: []discordEmbed{{
			Title:       fmt.Sprintf("%s is now live! 🔴", name),
			URL:         streamURL,
			Description: fmt.Sprintf("Come hang out → %s", streamURL),
			Color:       0x9146FF, // Twitch purple
		}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[DISCORD] failed to marshal go-live payload for %s: %v", login, err)
		return
	}

	client := &http.Client{Timeout: discordWebhookTimeout}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[DISCORD] go-live POST failed for %s: %v", login, err)
		return
	}
	defer resp.Body.Close()

	// Discord returns 204 No Content on success.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[DISCORD] go-live POST for %s returned status %d", login, resp.StatusCode)
		return
	}

	log.Printf("[DISCORD] posted go-live notification for %s", login)
}
