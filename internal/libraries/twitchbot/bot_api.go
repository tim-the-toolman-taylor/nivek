package twitchbot

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivekmiddleware"
)

// Notification message types
const (
	MESSAGE_TYPE_VERIFICATION = "webhook_callback_verification"
	MESSAGE_TYPE_NOTIFICATION = "notification"
	MESSAGE_TYPE_REVOCATION   = "revocation"
)

const HMAC_PREFIX = "sha256="

// Notification request headers
const (
	TWITCH_MESSAGE_ID        = "Twitch-Eventsub-Message-Id"
	TWITCH_MESSAGE_TIMESTAMP = "Twitch-Eventsub-Message-Timestamp"
	TWITCH_MESSAGE_SIGNATURE = "Twitch-Eventsub-Message-Signature"
	MESSAGE_TYPE             = "Twitch-Eventsub-Message-Type"
)

// defaultWebhookListenAddress is where the EventSub HTTP listener binds. It must
// be reachable from the Traefik gateway container over the docker bridge, so it
// binds all interfaces rather than loopback: Traefik routes
// peanutbudderbot.com/eventsub -> http://172.19.0.1:8090. Override with
// WEBHOOK_LISTEN_ADDRESS if the port or bridge subnet ever changes.
const defaultWebhookListenAddress = "0.0.0.0:8090"

type EventSubSubscriptionResponse struct {
	Challenge    string               `json:"challenge,omitempty"`
	Subscription SubscriptionResponse `json:"subscription"`
	// Event is present on notification message - keep raw until needed
	Event     json.RawMessage   `json:"event,omitempty"`
	Transport map[string]string `json:"transport"`
	CreatedAt string            `json:"created_at"`
}

type SubscriptionResponse struct {
	Id        string            `json:"id"`
	Status    string            `json:"status"`
	Type      string            `json:"type"`
	Cost      int               `json:"cost"`
	Version   string            `json:"version"`
	Condition map[string]string `json:"condition"`
}

// NewWebhookListener starts the Twitch EventSub HTTP listener in the background.
// It returns once the server goroutine is launched so bot startup is not
// blocked; the server runs for the lifetime of the process.
func NewWebhookListener(bot *Bot) {
	addr := os.Getenv("WEBHOOK_LISTEN_ADDRESS")
	if addr == "" {
		addr = defaultWebhookListenAddress
	}

	if os.Getenv("TWITCH_EVENTSUB_SECRET") == "" {
		log.Println("warning: TWITCH_EVENTSUB_SECRET is not set; eventsub signature verification will reject every request")
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.POST(
		api.TwitchWebhookSubscriptionRequest,
		newTwitchEventSubEndpoint(bot),
	)
	// core-api -> bot realtime commands, authenticated with the shared bot key.
	e.POST(
		api.PostBotJoinChannel,
		newJoinChannelEndpoint(bot),
		nivekmiddleware.NewHMACMiddleware("BOT_API_HMAC_KEY"),
	)

	go func() {
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Printf("eventsub webhook listener stopped: %v", err)
		}
	}()

	log.Printf("eventsub webhook listener started on %s%s", addr, api.TwitchWebhookSubscriptionRequest)
}

// newJoinChannelEndpoint lets core-api tell the bot to join a channel in
// realtime — e.g. a user who signs up while already live, whom EventSub's
// go-live transition would otherwise miss. Guarded by the shared HMAC
// middleware, so only core-api (holding BOT_API_HMAC_KEY) can trigger a join.
func newJoinChannelEndpoint(bot *Bot) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req struct {
			BroadcasterUserLogin string `json:"twitch_login"`
		}
		if err := c.Bind(&req); err != nil {
			log.Printf("join endpoint: failed to bind body: %s", err.Error())
			return c.NoContent(http.StatusBadRequest)
		}
		if req.BroadcasterUserLogin == "" {
			log.Printf("join endpoint: missing twitch_login")
			return c.NoContent(http.StatusBadRequest)
		}

		login := strings.ToLower(req.BroadcasterUserLogin)
		log.Printf("join endpoint: joining channel %s", login)
		bot.client.Join(login)

		return c.NoContent(http.StatusNoContent)
	}
}

func newTwitchEventSubEndpoint(bot *Bot) echo.HandlerFunc {
	return func(c echo.Context) error {
		secret := os.Getenv("TWITCH_EVENTSUB_SECRET")

		body, err := io.ReadAll(c.Request().Body)
		if err != nil {
			log.Printf("failed to read eventsub body: %v", err)
			return c.NoContent(http.StatusBadRequest)
		}
		// Restore body in case anything else reads it later.
		c.Request().Body = io.NopCloser(bytes.NewReader(body))

		message := getHmacMessage(c.Request(), string(body))
		computedHmac := fmt.Sprintf("%s%s", HMAC_PREFIX, getHmac(secret, message))
		if !verifyMessage(computedHmac, c.Request().Header.Get(TWITCH_MESSAGE_SIGNATURE)) {
			return c.NoContent(http.StatusForbidden)
		}

		var notification EventSubSubscriptionResponse
		if err := c.Bind(&notification); err != nil {
			log.Printf("failed to parse webhook subscription response: %s", err.Error())
			return c.NoContent(http.StatusBadRequest)
		}

		if MESSAGE_TYPE_NOTIFICATION == c.Request().Header.Get(MESSAGE_TYPE) {
			log.Printf("[WEBHOOK] type=%s", notification.Subscription.Type)

			switch notification.Subscription.Type {
			case "stream.online":
				handleGoLive(bot, &notification)

			case "stream.offline":
				handleGoOffline(bot, &notification)

			default:
				log.Printf("unrecognized webhook: %+v", notification)
			}

			return c.NoContent(http.StatusNoContent)
		}

		if MESSAGE_TYPE_VERIFICATION == c.Request().Header.Get(MESSAGE_TYPE) {
			return c.String(http.StatusOK, notification.Challenge)
		}

		if MESSAGE_TYPE_REVOCATION == c.Request().Header.Get(MESSAGE_TYPE) {
			log.Printf("eventsub %s notifications revoked: reason=%s condition=%v",
				notification.Subscription.Type,
				notification.Subscription.Status,
				notification.Subscription.Condition,
			)
			return c.NoContent(http.StatusNoContent)
		}

		return c.NoContent(http.StatusBadRequest)
	}
}

func handleGoLive(bot *Bot, notification *EventSubSubscriptionResponse) {
	var event struct {
		Id                   string `json:"id"`
		BroadcasterUserId    string `json:"broadcaster_user_id"`
		BroadcasterUserLogin string `json:"broadcaster_user_login"`
		BroadcasterUserName  string `json:"broadcaster_user_name"`
		Type                 string `json:"type"`
		StartedAt            string `json:"started_at"`
	}

	if err := json.Unmarshal(notification.Event, &event); err != nil {
		log.Printf("failed to unmarshal stream.online notification event %+v, %s", notification, err.Error())
		return
	}

	// A banished channel keeps its stream.online subscription on Twitch's side, so
	// we must not blindly rejoin on go-live. Permanent home channels and channels
	// already tracked in-memory are always fine. For anything else — which is
	// either a brand-new opted-in user (offline signup we haven't seen yet) or a
	// banished/opted-out channel — the authoritative signal is the DB bot_opt_in
	// flag, so ask core-api. Fail open (join) on error so a core-api blip can't
	// drop a legitimate go-live.
	login := strings.ToLower(event.BroadcasterUserLogin)
	if !bot.isPermanentChannel(login) && !bot.isTrackedChannel(login) {
		optedIn, err := bot.coreAPI.IsChannelOptedIn(login)
		if err != nil {
			log.Printf("[GOLIVE] opt-in check failed for %s, joining anyway: %v", login, err)
		} else if !optedIn {
			log.Printf("[GOLIVE] ignoring go-live for banished/opted-out channel %s", login)
			return
		}
	}

	// Synchronous: this mints the fresh per-stream key in users.stream_key, and
	// fetchAutoShoutChatters below filters on it — so it must land first.
	updateState(bot, &event.BroadcasterUserLogin, true)
	bot.client.Join(event.BroadcasterUserLogin)
	// Announce only on a genuine go-live webhook, not on boot-from-state joins.
	bot.say(strings.ToLower(event.BroadcasterUserLogin), "p nut budder is here!")
	// Fresh stream: reset per-stream tickers
	bot.fetchAutoShoutChatters(&event.BroadcasterUserId, &event.BroadcasterUserLogin)
	// reset every chatter's per-stream !dad allotment.
	bot.startDadStream(event.BroadcasterUserLogin, event.StartedAt)
	// Announce the go-live in Discord (no-op if DISCORD_WEBHOOK_URL is unset).
	go notifyDiscordGoLive(event.BroadcasterUserName, event.BroadcasterUserLogin)
	log.Printf("[WEBHOOK] %s is now live!", event.BroadcasterUserLogin)
}

func handleGoOffline(bot *Bot, notification *EventSubSubscriptionResponse) {
	var event struct {
		Id                   string `json:"id"`
		BroadcasterUserId    string `json:"broadcaster_user_id"`
		BroadcasterUserLogin string `json:"broadcaster_user_login"`
		BroadcasterUserName  string `json:"broadcaster_user_name"`
	}

	if err := json.Unmarshal(notification.Event, &event); err != nil {
		log.Printf("failed to unmarshal stream.offline notification event %+v, %s", notification, err.Error())
		return
	}

	go updateState(bot, &event.BroadcasterUserLogin, false)
	// Never leave the permanent home channels, even when they go offline.
	if !bot.isPermanentChannel(event.BroadcasterUserLogin) {
		bot.client.Depart(strings.ToLower(event.BroadcasterUserLogin))
	}
	// Stream over: drop this channel's !dad counters (event-driven cleanup).
	bot.endDadStream(event.BroadcasterUserLogin)
	log.Printf("[WEBHOOK] %s is now offline", event.BroadcasterUserLogin)
}

func updateState(bot *Bot, broadcasterUserLogin *string, isLive bool) {
	if err := bot.coreAPI.PushState(broadcasterUserLogin, isLive); err != nil {
		log.Printf("failed to push Broadcaster State update for broadcaster: %s - %s", *broadcasterUserLogin, err.Error())
	}
}

func getHmacMessage(request *http.Request, body string) string {
	return fmt.Sprintf("%s%s%s",
		request.Header.Get(strings.ToLower(TWITCH_MESSAGE_ID)),
		request.Header.Get(strings.ToLower(TWITCH_MESSAGE_TIMESTAMP)),
		body,
	)
}

func getHmac(secret, message string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyMessage(computedHmac, verifySignature string) bool {
	return hmac.Equal([]byte(computedHmac), []byte(verifySignature))
}
