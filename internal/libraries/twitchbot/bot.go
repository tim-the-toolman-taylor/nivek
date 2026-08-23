package twitchbot

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gempir/go-twitch-irc/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overseer"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/promo"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitcheventsub"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

const botCreatorChannel = "timallenfanclubofficial"

type commandHandler func(b *Bot, message *chatMessageEvent)

// builtinRegistry is the compiled set of behaviors the bot can perform, keyed
// by the stable handler_key stored in nivek.command. This is the ONLY place a
// handler_key becomes code. Keys MUST match the handler_key column values
// seeded into the command table; a mismatch surfaces as an orphan error in
// getCommands at boot rather than a silent no-op at dispatch.
var builtinRegistry = map[string]commandHandler{
	"banish":      (*Bot).handleBanishCommand,
	"bread":       (*Bot).handleBreadCommand,
	"dad_roll":    (*Bot).handleDadCommand,
	"fish":        (*Bot).handleFishCommand,
	"lurk":        (*Bot).handleLurkCommand,
	"join_me":     (*Bot).handleJoinCommand,
	"pb_commands": (*Bot).handlePbCommandsCommand,
	"new_promo":   (*Bot).handleNewPromoCommand,
}

type Config struct {
	BotUsername     string
	BotOAuth        string
	Channels        []user.User // Changed from single Channel to multiple Channels
	StoragePath     string
	Timezone        string
	ExecutorWSURL   string // e.g. ws://192.168.1.X:8123/ws
	OverseerHmacKey string // hex-encoded HMAC key, shared with the executor

	// TokenProvider, when non-nil, supplies a fresh user access token (WITHOUT
	// the "oauth:" prefix) before each IRC (re)connect, so an expiring token is
	// renewed automatically. Nil falls back to the static BotOAuth.
	TokenProvider func(context.Context) (string, error)
}

type sayRequest struct {
	broadcasterId string // ID of broadcaster whose channel the message will be sent to
	message       string
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
	coreAPI        api.CoreAPIClient
	httpClient     *http.Client
	twitchClient   *twitcheventsub.Client
	overseerClient *overseer.Client
	sayQueue       chan sayRequest

	// tokenProvider, when non-nil, returns a fresh IRC access token before each
	// (re)connect. See Config.TokenProvider.
	tokenProvider func(context.Context) (string, error)

	// channelsMu guards all access to config.Channels and the in-flight claim sets
	// below, so the self-heal and !joinme goroutines mutate the channel list
	// exactly once each and never race a concurrent read. healInFlight is keyed by
	// lowercased Twitch login; joinInFlight by Twitch user ID.
	channelsMu   sync.Mutex
	healInFlight map[string]bool
	joinInFlight map[string]bool

	commands map[string]commandHandler

	// dadMu guards dadUsage, the per-stream/per-chatter !dad rate-limit counters.
	// Populated on stream.online, evicted on stream.offline (see dad_limit.go).
	// Keyed by lowercased channel login.
	dadMu    sync.Mutex
	dadUsage map[string]*dadStreamUsage

	autoShout map[string][]string

	// liveMu guards live, the set of channels currently broadcasting (keyed by
	// lowercased login). Seeded from IsLive at boot and flipped by the
	// stream.online/offline webhooks. The promo scheduler consults it so recurring
	// messages only post to live chats — permanent home channels count as always
	// live (see isChannelLive).
	liveMu sync.Mutex
	live   map[string]bool
}

func NewBot(
	coreAPI api.CoreAPIClient,
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

	// Create Twitch IRC client @TODO::convert to websocket send-chat-message api
	client := twitch.NewClient(config.BotUsername, config.BotOAuth)
	client.IrcAddress = "irc.chat.twitch.tv:6697"
	client.TLS = true

	cmds, err := getGlobalEnabledCommands(coreAPI)
	if err != nil {
		panic(fmt.Sprintf("unable to load commands! %s", err.Error()))
	}

	bot := &Bot{
		client:         client,
		config:         config,
		counters:       counters,
		location:       loc,
		coreAPI:        coreAPI,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		twitchClient:   twitchClient,
		overseerClient: overseerCli,
		tokenProvider:  config.TokenProvider,
		healInFlight:   make(map[string]bool),
		joinInFlight:   make(map[string]bool),
		commands:       cmds,

		dadUsage: make(map[string]*dadStreamUsage),
		live:     make(map[string]bool),
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
	b.autoShout = make(map[string][]string, len(b.config.Channels))
	for _, channel := range b.config.Channels {
		if channel.TwitchLogin != nil {
			// Seed live-state so the promo scheduler knows which channels are
			// broadcasting from the moment it starts, before any webhook fires.
			b.setLive(*channel.TwitchLogin, channel.IsLive)
		}
		if channel.TwitchLogin != nil && channel.IsLive {
			b.client.Join(*channel.TwitchLogin)
			log.Printf("Joining channel: %s", *channel.TwitchLogin)

			if channel.TwitchID != nil {
				go b.fetchAutoShoutChatters(channel.TwitchID, channel.TwitchLogin)
				// Restore per-stream !dad counters so a restart mid-stream doesn't
				// hand every chatter a fresh allotment.
				go b.rehydrateDadUsage(*channel.TwitchLogin, *channel.TwitchID)
			}
		}

		// legacy users or users who haven't passed twitch get-users fetch
		// these users are not eligible for autoshout
		if channel.TwitchLogin == nil && len(channel.Username) > 0 {
			b.client.Join(strings.ToLower(channel.Username))
			log.Printf("Joining legacy user channel: %s", channel.Username)
		}
	}

	// The creator's channel and the bot's own channel are permanent: always
	// present, online or offline, and never departed/banished. Join is idempotent.
	b.client.Join(botCreatorChannel)
	b.client.Join(strings.ToLower(b.config.BotUsername))

	// Start reset timer
	go b.counters.StartResetTimer(ctx)

	// setup webhook listener
	NewWebhookListener(b)

	// Start the DF welcome/orientation announcer in dfCommandChannel.
	// go b.runDFWelcomeLoop(ctx) // temporarily disabled while bot is migrated from Pi -> VPS

	go b.runPromotionMessageLoop(ctx)

	// Start IRC client with panic recovery and auto-reconnect
	go b.connectWithPanicRecovery(ctx)

	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

// promoPollInterval is how often the scheduler re-reads the promo set from
// core-api and evaluates which are due. It bounds both how quickly a newly
// created/edited promo takes effect and the scheduling granularity, so it should
// stay well below the minimum promo interval (60s).
const promoPollInterval = 30 * time.Second

// runPromotionMessageLoop drives every recurring "promo" message. The DB (via
// core-api) is the single source of truth: each tick we fetch all enabled promos
// and post the ones whose interval has elapsed. Per-promo timing is held in
// memory (lastPosted, keyed by promo id) rather than the DB — losing it on
// restart only means each promo waits one fresh interval, never a burst.
//
// A promo only posts while its channel is live (permanent home channels always
// count as live). While a channel is offline its clock is continuously reset, so
// promos start a fresh interval when the stream comes online instead of firing a
// backlog all at once.
func (b *Bot) runPromotionMessageLoop(ctx context.Context) {
	ticker := time.NewTicker(promoPollInterval)
	defer ticker.Stop()

	lastPosted := make(map[int]time.Time)

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			promos, err := b.coreAPI.GetActivePromos()
			if err != nil {
				log.Printf("[PROMO] failed to fetch promos: %v", err)
				continue
			}

			seen := make(map[int]bool, len(promos))
			for _, p := range promos {
				seen[p.Id] = true

				if !b.isChannelLive(p.Channelname) {
					// Clock only runs while live: reset each offline tick so the
					// first post lands one full interval after going live, not a
					// pile of overdue posts the instant the stream starts.
					lastPosted[p.Id] = now
					continue
				}

				last, known := lastPosted[p.Id]
				if !known {
					// First time we've seen this promo (e.g. just created, or the
					// channel just went live): start its clock now, don't post yet.
					lastPosted[p.Id] = now
					continue
				}

				interval := p.IntervalSeconds
				if interval < promo.MinIntervalSeconds {
					interval = promo.MinIntervalSeconds
				}
				if now.Sub(last) < time.Duration(interval)*time.Second {
					continue
				}

				b.say(p.Channelname, &p.Message)
				lastPosted[p.Id] = now
				log.Printf("[PROMO] posted #%d to %s", p.Id, p.Channelname)
			}

			// Forget promos that were deleted or disabled so their timing state
			// can't linger and their ids can be reused cleanly.
			for id := range lastPosted {
				if !seen[id] {
					delete(lastPosted, id)
				}
			}
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
			// Refresh the IRC token before (re)connecting so an expired access
			// token is renewed automatically. On failure we log and connect with
			// whatever token the client already holds — never worse than before.
			if b.tokenProvider != nil {
				if tok, err := b.tokenProvider(ctx); err != nil {
					log.Printf("token refresh failed, using existing IRC token: %v", err)
				} else {
					b.client.SetIRCToken("oauth:" + tok)
				}
			}
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

// getCommands builds the dispatch map (trigger -> handler) by joining the
// command rows from core-api against builtinRegistry: the DB supplies
// trigger -> handler_key, the registry supplies handler_key -> func.
// Custom commands (no compiled handler) and disabled commands are skipped.
// An unknown handler_key is a boot-time error so a bad seed fails loud
// instead of silently dropping a command at dispatch.
func getGlobalEnabledCommands(coreAPI api.CoreAPIClient) (map[string]commandHandler, error) {
	rows, err := coreAPI.GetGlobalEnabledCommands()
	if err != nil {
		return nil, err
	}

	cmds := make(map[string]commandHandler, len(rows))
	for _, row := range rows {
		if row.Kind != "builtin" || !row.Enabled {
			continue
		}
		if row.HandlerKey == nil {
			return nil, fmt.Errorf("builtin command %q has null handler_key", row.Trigger)
		}
		handler, ok := builtinRegistry[*row.HandlerKey]
		if !ok {
			return nil, fmt.Errorf("command %q: unknown handler_key %q", row.Trigger, *row.HandlerKey)
		}
		cmds[row.Trigger] = handler
	}
	return cmds, nil
}
