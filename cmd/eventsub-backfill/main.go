// eventsub-backfill is a one-shot CLI that creates stream.online EventSub
// webhook subscriptions for every user with a twitch_id already in Postgres.
//
// Intended to be run manually on prod after deploying the EventSub callback
// (e.g. ssh + binary with the same env as core-api). Safe to re-run: Helix 409
// (already subscribed) is treated as success.
//
// Usage:
//
//	eventsub-backfill [-dry-run] [-delay 200ms]
//
// Required env (same names as core-api):
//
//	TWITCH_CLIENT_ID
//	TWITCH_CLIENT_SECRET
//	TWITCH_EVENTSUB_SECRET
//	POSTGRES_HOST, POSTGRES_USERNAME, POSTGRES_PASSWORD, POSTGRES_DB
//
// Optional:
//
//	POSTGRES_PORT (default 5432)
//	TWITCH_EVENTSUB_CALLBACK (default https://peanutbudderbot.com/api/twitch/eventsub)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitcheventsub"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "list users only; do not call Twitch")
	delay := flag.Duration("delay", 200*time.Millisecond, "pause between Helix create calls")
	flag.Parse()

	_ = godotenv.Load()

	clientID := mustEnv("TWITCH_CLIENT_ID")
	clientSecret := mustEnv("TWITCH_CLIENT_SECRET")
	eventSubSecret := mustEnv("TWITCH_EVENTSUB_SECRET")
	callback := envOr("TWITCH_EVENTSUB_CALLBACK", fmt.Sprintf("https://peanutbudderbot.com%s", api.TwitchWebhookSubscriptionRequest))

	ctx := context.Background()

	users, err := loadUsersWithTwitchID()
	if err != nil {
		log.Fatalf("load users: %v", err)
	}
	log.Printf("found %d users with twitch_id", len(users))

	if *dryRun {
		for _, u := range users {
			if u.TwitchID == nil || u.TwitchLogin == nil {
				log.Printf("user %d missing TwitchID or TwitchLogin. Skipping...", u.Id)
				continue
			}
			log.Printf("dry-run: would subscribe stream.online user_id=%d username=%s twitch_id=%s",
				u.Id, *u.TwitchLogin, *u.TwitchID)
		}
		log.Printf("dry-run complete (%d users)", len(users))
		return
	}

	client, err := twitcheventsub.NewClient(twitcheventsub.Config{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		EventSubSecret: eventSubSecret,
		CallbackURL:    callback,
	})
	if err != nil {
		log.Fatalf("eventsub client: %v", err)
	}

	webhooks := map[string]func(context.Context, string) (twitcheventsub.SubscribeResult, error){
		"stream.online":  client.SubscribeStreamOnline,
		"stream.offline": client.SubscribeStreamOffline,
	}

	var ok, exists, failed int
	for i, u := range users {
		if u.TwitchID == nil || u.TwitchLogin == nil {
			log.Printf("user %d missing TwitchID or TwitchLogin. Skipping...", u.Id)
			continue
		}
		twitchID := *u.TwitchID

		for webhookName, webhookFunc := range webhooks {

			result, err := webhookFunc(ctx, twitchID)
			if err != nil {
				failed++
				log.Printf("[%d/%d] FAIL %s user_id=%d username=%s twitch_id=%s err=%v",
					i+1, len(users), webhookName, u.Id, *u.TwitchLogin, twitchID, err)
			} else if result.AlreadyExists() {
				exists++
				log.Printf("[%d/%d] already-subscribed user_id=%d username=%s twitch_id=%s",
					i+1, len(users), u.Id, *u.TwitchLogin, twitchID)
			} else if result.OK() {
				ok++
				log.Printf("[%d/%d] subscribed user_id=%d username=%s twitch_id=%s status=%d",
					i+1, len(users), u.Id, *u.TwitchLogin, twitchID, result.StatusCode)
			} else {
				failed++
				log.Printf("[%d/%d] FAIL user_id=%d username=%s twitch_id=%s status=%d body=%s",
					i+1, len(users), u.Id, *u.TwitchLogin, twitchID, result.StatusCode, string(result.Body))
			}

		}

		if i < len(users)-1 && *delay > 0 {
			time.Sleep(*delay)
		}
	}

	log.Printf("done: subscribed=%d already_existed=%d failed=%d total=%d", ok, exists, failed, len(users))
	if failed > 0 {
		os.Exit(1)
	}
}

func loadUsersWithTwitchID() ([]user.User, error) {
	coreAPIURL := envOr("CORE_API_URL", "")
	botHmacKey := envOr("BOT_API_HMAC_KEY", "")
	if coreAPIURL == "" || botHmacKey == "" {
		log.Fatal("Missing required environment variables: CORE_API_URL, BOT_API_HMAC_KEY")
	}

	coreAPI, err := api.NewCoreAPIClient(coreAPIURL, botHmacKey)
	if err != nil {
		log.Fatalf("Failed to create core-api client: %v", err)
	}

	users, err := coreAPI.GetActiveChannels()
	if err != nil {
		log.Fatalf("Failed to fetch active channels from core-api: %v", err)
	}
	if len(users) == 0 {
		log.Fatal("No active users returned by core-api")
	}

	// Defensive: drop any row with nil/empty twitch_id (Cond may vary by adapter).
	out := make([]user.User, 0, len(users))
	for _, u := range users {
		if u.TwitchID != nil && *u.TwitchID != "" {
			out = append(out, u)
		}
	}
	return out, nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env %s", key)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
