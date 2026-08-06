package twitchbot

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/gempir/go-twitch-irc/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overseer"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitcheventsub"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

const botCreatorChannel = "timallenfanclubofficial"

type commandHandler func(b *Bot, message *twitch.PrivateMessage)

type Config struct {
	BotUsername     string
	BotOAuth        string
	Channels        []user.User // Changed from single Channel to multiple Channels
	StoragePath     string
	Timezone        string
	ExecutorWSURL   string // e.g. ws://192.168.1.X:8123/ws
	OverseerHmacKey string // hex-encoded HMAC key, shared with the executor
}

type sayRequest struct {
	channel string
	message string
}

// Bot has no direct Postgres dependency. All persistent state goes through
// CoreAPIClient → HMAC-authed RPC → core-api → DB. That way a compromised Pi
// can't drain prod data: it only has bot-scoped API capability, not raw DB
// credentials.
type Bot struct {
	client         *twitch.Client
	config         Config
	counters       *CounterManager
	location       *time.Location
	coreAPI        *api.CoreAPIClient
	twitchClient   *twitcheventsub.Client
	overseerClient *overseer.Client
	sayQueue       chan sayRequest
}

func NewBot(
	coreAPI *api.CoreAPIClient,
	twitchClient *twitcheventsub.Client,
	config Config,
) (*Bot, error) {

	// Load timezone
	loc, err := time.LoadLocation(config.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %s: %w", config.Timezone, err)
	}

	// Create counter manager
	counters, err := NewCounterManager(config.StoragePath, loc)
	if err != nil {
		return nil, fmt.Errorf("failed to create counter manager: %w", err)
	}

	// overseer (DF Twitch-plays) WebSocket client to the executor
	hmacKey, err := hex.DecodeString(config.OverseerHmacKey)
	if err != nil {
		return nil, fmt.Errorf("invalid OVERSEER_HMAC_KEY hex: %w", err)
	}
	overseerCli := overseer.NewClient(config.ExecutorWSURL, hmacKey)

	// Create Twitch IRC client
	client := twitch.NewClient(config.BotUsername, config.BotOAuth)
	client.IrcAddress = "irc.chat.twitch.tv:6697"
	client.TLS = true

	bot := &Bot{
		client:         client,
		config:         config,
		counters:       counters,
		location:       loc,
		coreAPI:        coreAPI,
		twitchClient:   twitchClient,
		overseerClient: overseerCli,
	}

	bot.sayQueue = make(chan sayRequest, 64)
	go bot.senderLoop()

	// Register message handler
	client.OnPrivateMessage(bot.handleMessage)

	client.OnNoticeMessage(func(m twitch.NoticeMessage) {
		log.Printf("[NOTICE] [%s] msg-id=%s: %s", m.Channel, m.MsgID, m.Message)
	})

	// Log connection events
	client.OnConnect(func() {
		log.Printf("Connected to Twitch IRC as %s", config.BotUsername)
	})

	return bot, nil
}

func (b *Bot) Start(ctx context.Context) error {
	for _, channel := range b.config.Channels {
		// @TODO::remove channel.TwitchLogin once self-heal system finishes
		if channel.TwitchLogin != nil && channel.IsLive {
			b.client.Join(*channel.TwitchLogin)
			b.say(*channel.TwitchLogin, "p nut budder is here!")
			log.Printf("Joining channel: %s", *channel.TwitchLogin)
		}

		if channel.TwitchLogin == nil && len(channel.Username) > 0 { // this is needed to get legacy users to authenticate for latest updates
			b.client.Join(strings.ToLower(channel.Username))
			b.say(strings.ToLower(channel.Username), "p nut budder is here!")
			log.Printf("Joining legacy user channel: %s", channel.Username)
		}
	}

	// Start reset timer
	go b.counters.StartResetTimer(ctx)

	// setup webhook listener
	NewWebhookListener(b)

	// Start the DF welcome/orientation announcer in dfCommandChannel.
	// go b.runDFWelcomeLoop(ctx) // temporarily disabled while bot is migrated from Pi -> VPS

	// Start IRC client with panic recovery and auto-reconnect
	go b.connectWithPanicRecovery(ctx)

	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

func (b *Bot) handleMessage(message twitch.PrivateMessage) {
	// Normalize message
	msg := strings.TrimSpace(strings.ToLower(message.Message))
	chattername := message.User.Name

	var commands = map[string]commandHandler{
		"!bread":  (*Bot).handleBreadCommand,
		"!fish":   (*Bot).handleFishCommand,
		"!dad":    func(b *Bot, message *twitch.PrivateMessage) { b.say(message.Channel, "still out getting milk!") },
		"!lurk":   (*Bot).handleLurkCommand,
		"!joinme": (*Bot).handleJoinCommand,
	}

	if slices.Contains(slices.Collect(maps.Keys(commands)), msg) {
		log.Printf("[CMD-RECV] [%s] %s: %q", message.Channel, chattername, msg)
	}

	// if b.autoShout.OnMessage(channel, chattername) {
	// 	b.client.Say(channel, fmt.Sprintf("!so @%s", chattername))
	// }

	// !DF takes arguments — handle separately from the exact-match commands below
	if msg == "!df" || strings.HasPrefix(msg, "!df ") {
		if message.Channel != botCreatorChannel {
			return
		}
		args := strings.TrimSpace(strings.TrimPrefix(msg, "!df"))
		b.handleDFCommand(message.Message, args, chattername, message.Channel)
		return
	}

	// Check for commands
	for cmd, handler := range commands {
		if strings.Contains(msg, cmd) {
			handler(b, &message)
		}
	}

	// self-heal for users that pre-date webhook system
	if message.User.Name == message.Channel {
		go b.isLegacyChannel()
	}
}

func (b *Bot) isLegacyChannel() {
	for _, c := range b.config.Channels {
		if c.TwitchID == nil {
			// we have a legacy user - patch
			
		}
	}
}

func (b *Bot) senderLoop() {
	for req := range b.sayQueue {
		b.client.Say(req.channel, req.message)
		time.Sleep(1500 * time.Millisecond)
	}
}

func (b *Bot) say(channel, message string) {
	b.sayQueue <- sayRequest{channel, message}
}

func (b *Bot) connectWithPanicRecovery(ctx context.Context) {
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from Twitch IRC panic: %v", r)
				}
			}()
			if err := b.client.Connect(); err != nil {
				log.Printf("Error connecting to Twitch: %v", err)
			}
		}()

		select {
		case <-ctx.Done():
			return
		default:
			log.Println("Reconnecting to Twitch IRC in 5 seconds...")
			time.Sleep(5 * time.Second)
		}
	}
}

func (b *Bot) Stop() {
	log.Println("Disconnecting from Twitch...")
	b.client.Disconnect()

	// Save counters one last time
	if err := b.counters.Save(); err != nil {
		log.Printf("Error saving counters on shutdown: %v", err)
	}
}

func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// abs returns |n|. Used by dfSuccessReply when formatting a Mine
// action's dimensions for chat — Region.Max can be either >= or < Min
// depending on which corner the chatter typed first. Duplicated rather
// than imported because the overseer package's copy is unexported and
// the helper is too small to justify exposing or sharing.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
