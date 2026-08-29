package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitchauth"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitchbot"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitcheventsub"
)

// The bot has no Postgres dependency. It reaches all persistent state through
// core-api over HMAC-authed HTTPS (see internal/libraries/twitchbot/
// coreapiclient.go). Required Pi-side env:
//
//	CORE_API_URL       — e.g. https://peanutbudderbot.com
//	BOT_API_HMAC_KEY   — hex, must match the key core-api validates against
//	TWITCH_BOT_USERNAME, TWITCH_BOT_OAUTH
//	EXECUTOR_WS_URL, OVERSEER_HMAC_KEY (for DF Twitch-plays)
func main() {
	coreAPIURL := getEnv("CORE_API_URL", "")
	botHmacKey := getEnv("BOT_API_HMAC_KEY", "")
	if coreAPIURL == "" || botHmacKey == "" {
		log.Fatal("Missing required environment variables: CORE_API_URL, BOT_API_HMAC_KEY")
	}

	coreAPI, err := api.NewCoreAPIClient(coreAPIURL, botHmacKey)
	if err != nil {
		log.Fatalf("Failed to create core-api client: %v", err)
	}

	clientID := getEnv("TWITCH_CLIENT_ID", "")
	clientSecret := getEnv("TWITCH_CLIENT_SECRET", "")
	refreshToken := getEnv("TWITCH_BOT_REFRESH", "")

	config := twitchbot.Config{
		BotUsername:     getEnv("TWITCH_BOT_USERNAME", ""),
		BotId:           "1322716097",
		BotOAuth:        getEnv("TWITCH_BOT_OAUTH", ""),
		ClientID:        clientID,
		Channels:        getLiveChannels(coreAPI),
		StoragePath:     getEnv("TWITCH_STORAGE_PATH", "./data/twitch-counters.json"),
		Timezone:        getEnv("TWITCH_TIMEZONE", "America/New_York"),
		ExecutorWSURL:   getEnv("EXECUTOR_WS_URL", ""),
		OverseerHmacKey: getEnv("OVERSEER_HMAC_KEY", ""),
	}

	// Auto-refresh the IRC token when client creds + a refresh token are present
	// (all three must belong to the same Twitch app). Falls back to the static
	// TWITCH_BOT_OAUTH otherwise.
	if clientID != "" && clientSecret != "" && refreshToken != "" {
		config.TokenProvider = twitchauth.NewRefresher(clientID, clientSecret, refreshToken).Token
		log.Println("Twitch IRC token auto-refresh enabled")
	} else {
		log.Println("Twitch IRC token auto-refresh disabled (set TWITCH_CLIENT_ID, TWITCH_CLIENT_SECRET, TWITCH_BOT_REFRESH to enable); using static TWITCH_BOT_OAUTH")
	}

	// App access token for Helix Send Chat Message so the bot's messages earn the
	// Chat Bot Badge. Only client creds are needed (client-credentials grant);
	// sends the app token can't authorize fall back to the user token.
	if clientID != "" && clientSecret != "" {
		config.AppTokenProvider = twitchauth.NewAppTokenSource(clientID, clientSecret).Token
		log.Println("Chat Bot Badge send path enabled (app access token)")
	} else {
		log.Println("Chat Bot Badge send path disabled (set TWITCH_CLIENT_ID + TWITCH_CLIENT_SECRET to enable); sending with user token")
	}

	if config.BotUsername == "" || len(config.Channels) == 0 {
		log.Fatal("Missing required environment variables: TWITCH_BOT_USERNAME (and core-api must return at least one channel)")
	}
	if config.BotOAuth == "" && config.TokenProvider == nil {
		log.Fatal("Need TWITCH_BOT_OAUTH, or TWITCH_CLIENT_ID + TWITCH_CLIENT_SECRET + TWITCH_BOT_REFRESH for auto-refresh")
	}

	twitchClient, err := twitcheventsub.NewClient(twitcheventsub.Config{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		EventSubSecret: getEnv("TWITCH_EVENTSUB_SECRET", ""),
	})

	bot, err := twitchbot.NewBot(coreAPI, twitchClient, config)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- bot.Start(ctx)
	}()

	select {
	case <-sigChan:
		log.Println("Received shutdown signal, gracefully stopping bot...")
		cancel()
		bot.Stop()
	case err := <-errChan:
		if err != nil {
			log.Fatalf("Bot error: %v", err)
		}
	}

	log.Println("Bot stopped successfully")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

var validTwitchLogin = regexp.MustCompile(`^[a-z0-9_]{4,25}$`)

// getChannelNames fetches the active-user list from core-api and filters to
// valid Twitch logins. The filter stays bot-side because it's an IRC-protocol
// concern (4-25 chars, lowercase, [a-z0-9_]), not something the API should
// gatekeep.
func getLiveChannels(coreAPI api.CoreAPIClient) []twitchbot.BotUser {
	users, err := coreAPI.GetActiveChannels()
	if err != nil {
		log.Fatalf("Failed to fetch active channels from core-api: %v", err)
	}
	if len(users) == 0 {
		log.Fatal("No active users returned by core-api")
	}

	channels := make([]twitchbot.BotUser, 0, len(users))
	for _, u := range users {
		if u.TwitchLogin == nil || !validTwitchLogin.MatchString(*u.TwitchLogin) {
			continue
		}
		// hasPrivs is seeded later from Twitch's EventSub state (hydrateChatReadPrivs).
		channels = append(channels, twitchbot.BotUser{User: u})
	}
	if len(channels) == 0 {
		log.Fatal("No active users with valid Twitch logins returned by core-api")
	}
	return channels
}
