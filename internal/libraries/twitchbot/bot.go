package twitchbot

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gempir/go-twitch-irc/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overseer"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitcheventsub"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

const botCreatorChannel = "timallenfanclubofficial"

const botherMessageInterval = 1 * time.Hour

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

	// botherCancel maps a lowercased legacy channel -> the cancel func for its
	// hourly "please authenticate" nag loop, so StopBother can end that loop the
	// instant the user authenticates. Guarded by botherMu: written by Start
	// (spawn) and the /internal/stop-bother handler goroutine (stop).
	botherMu     sync.Mutex
	botherCancel map[string]context.CancelFunc
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
		botherCancel:   make(map[string]context.CancelFunc),
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
		if channel.TwitchLogin != nil && channel.IsLive {
			b.client.Join(*channel.TwitchLogin)
			log.Printf("Joining channel: %s", *channel.TwitchLogin)
		}

		if channel.TwitchLogin == nil && len(channel.Username) > 0 { // this is needed to get legacy users to authenticate for latest updates
			b.client.Join(strings.ToLower(channel.Username))
			log.Printf("Joining legacy user channel: %s", channel.Username)
		}
	}

	// Start reset timer
	go b.counters.StartResetTimer(ctx)

	// setup webhook listener
	NewWebhookListener(b)

	// Start the DF welcome/orientation announcer in dfCommandChannel.
	// go b.runDFWelcomeLoop(ctx) // temporarily disabled while bot is migrated from Pi -> VPS
	for _, channel := range b.config.Channels {
		if channel.TwitchLogin == nil && len(channel.Username) > 0 {
			b.startBotherLoop(ctx, strings.ToLower(channel.Username))
		}
	}

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
	channel := message.Channel

	var commands = map[string]commandHandler{
		"!bread": (*Bot).handleBreadCommand,
		"!fish":  (*Bot).handleFishCommand,
		"!dad":   func(b *Bot, message *twitch.PrivateMessage) { b.say(message.Channel, "still out getting milk!") },
		"!lurk":  (*Bot).handleLurkCommand,
		"!join":  (*Bot).handleJoinCommand,
	}

	if slices.Contains(slices.Collect(maps.Keys(commands)), msg) {
		log.Printf("[CMD-RECV] [%s] %s: %q", channel, chattername, msg)
	}

	// if b.autoShout.OnMessage(channel, chattername) {
	// 	b.client.Say(channel, fmt.Sprintf("!so @%s", chattername))
	// }

	// !DF takes arguments — handle separately from the exact-match commands below
	if msg == "!df" || strings.HasPrefix(msg, "!df ") {
		if channel != botCreatorChannel {
			return
		}
		args := strings.TrimSpace(strings.TrimPrefix(msg, "!df"))
		b.handleDFCommand(message.Message, args, chattername, channel)
		return
	}

	// Check for commands
	for cmd, handler := range commands {
		if strings.Contains(msg, cmd) {
			handler(b, &message)
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

// startBotherLoop launches the hourly nag goroutine for a legacy channel and
// registers its cancel func so StopBother can end it the moment the user
// authenticates. channel must already be lowercased. If a loop is somehow
// already registered for the channel it's cancelled first, so we never run two.
func (b *Bot) startBotherLoop(ctx context.Context, channel string) {
	b.botherMu.Lock()
	if existing, ok := b.botherCancel[channel]; ok {
		existing()
	}
	loopCtx, cancel := context.WithCancel(ctx)
	b.botherCancel[channel] = cancel
	b.botherMu.Unlock()

	go b.runBotUpdateBotherMessageLoop(loopCtx, channel)
}

// StopBother ends the hourly nag loop for a channel — called when the user
// authenticates. No-op if there's no active loop for that channel, so it's safe
// to call for returning/non-legacy users or after a restart.
func (b *Bot) StopBother(channel string) {
	channel = strings.ToLower(channel)

	b.botherMu.Lock()
	cancel, ok := b.botherCancel[channel]
	if ok {
		delete(b.botherCancel, channel)
	}
	b.botherMu.Unlock()

	if ok {
		cancel()
		log.Printf("stopped bother loop for %s (authenticated)", channel)
	}
}

func (b *Bot) runBotUpdateBotherMessageLoop(ctx context.Context, channel string) {
	ticker := time.NewTicker(botherMessageInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.say(
				channel,
				"New features are available for this bot - message @timallenfanclubofficial to get set up.",
			)
		}
	}
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
