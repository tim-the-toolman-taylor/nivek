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
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/commands"
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
	"join_me":     (*Bot).handleJoinCommand,
	"lurk":        (*Bot).handleLurkCommand,
	"pb_commands": (*Bot).handlePbCommandsCommand,
	"new_promo":   (*Bot).handleNewPromoCommand,
	"stalk":       (*Bot).handleStalkCommand,
}

type BotUser struct {
	user.User
	hasPrivs bool
}

type Config struct {
	BotUsername     string
	BotId           string
	BotOAuth        string
	ClientID        string    // Twitch app client id; Helix Send Chat Message requires it
	Channels        []BotUser // Changed from single Channel to multiple Channels
	StoragePath     string
	Timezone        string
	ExecutorWSURL   string // e.g. ws://192.168.1.X:8123/ws
	OverseerHmacKey string // hex-encoded HMAC key, shared with the executor

	// TokenProvider, when non-nil, supplies a fresh user access token (WITHOUT
	// the "oauth:" prefix) before each IRC (re)connect and for Helix Send Chat
	// Message. Nil falls back to the static BotOAuth.
	TokenProvider func(context.Context) (string, error)
}

type sayRequest struct {
	broadcasterId string
	message       string
}

type ircSayRequest struct {
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
	coreAPI        api.CoreAPIClient
	httpClient     *http.Client
	twitchClient   twitcheventsub.TwitchEventSubClient
	overseerClient *overseer.Client
	sayQueue       chan sayRequest
	ircSayQueue    chan ircSayRequest

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

	// customMu guards customCommands, the per-channel custom ("channel"-scoped)
	// command sets. Outer key is the lowercased channel login (what handleMessage
	// dispatches on); inner key is the lowercased trigger. Loaded on stream.online
	// / boot-if-live, evicted on stream.offline (see custom_commands.go).
	customMu       sync.Mutex
	customCommands map[string]map[string]commands.Commands

	// stalkMu guards stalk, the per-channel !stalk watches. Outer key is the
	// lowercased channel login. A channel is present only when it has a
	// configured target — loaded on stream.online / boot-if-live, evicted on
	// stream.offline (see cmd_stalk.go). handleMessage records a last message
	// only for that target chatter, not for the rest of chat.
	stalkMu sync.Mutex
	stalk   map[string]*stalkWatch
}

func NewBot(
	coreAPI api.CoreAPIClient,
	twitchClient twitcheventsub.TwitchEventSubClient,
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

		dadUsage:       make(map[string]*dadStreamUsage),
		live:           make(map[string]bool),
		customCommands: make(map[string]map[string]commands.Commands),
		stalk:          make(map[string]*stalkWatch),
	}

	bot.sayQueue = make(chan sayRequest, 64)
	bot.ircSayQueue = make(chan ircSayRequest, 64)
	go bot.ircsenderLoop()
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
				// This channel is already live at boot, so load its custom commands
				// now — a mid-stream restart won't get a fresh stream.online webhook.
				go b.loadCustomCommands(*channel.TwitchID, *channel.TwitchLogin)
			}
			// Same live-session load as custom commands: pull this channel's
			// !stalk target if one is configured.
			go b.loadStalkTarget(*channel.TwitchLogin)
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

	// Seed chat-read privileges from Twitch's live EventSub state so the "mod me"
	// nudge stays quiet in channels that already granted it. Off the boot path.
	go b.hydrateChatReadPrivs(ctx)

	// Start the DF welcome/orientation announcer in dfCommandChannel.
	// go b.runDFWelcomeLoop(ctx) // temporarily disabled while bot is migrated from Pi -> VPS

	go b.runPromotionMessageLoop(ctx)

	// Start IRC client with panic recovery and auto-reconnect
	go b.connectWithPanicRecovery(ctx)

	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

func (b *Bot) handleMessage(message twitch.PrivateMessage) {
	if strings.EqualFold(message.User.Name, b.config.BotUsername) {
		return
	}

	msg := strings.TrimSpace(strings.ToLower(message.Message))
	if isBanishCommand(msg) {
		// EventSub already handles !banish in channels that have granted chat-read.
		if !b.channelHasPrivs(message.Channel) {
			b.handleIRCBanish(message)
		}
		return
	}

	// IRC no longer executes other commands. Every remaining line in a channel
	// that has not granted chat-read gets the "mod me" nudge. Home channels
	// are skipped. Stops once granted.
	if b.isPermanentChannel(message.Channel) || b.channelHasPrivs(message.Channel) {
		return
	}
	go b.ensureChatReadPrivs(message.Channel)
}

func (b *Bot) ircsenderLoop() {
	for req := range b.ircSayQueue {
		b.client.Say(req.channel, req.message)
		time.Sleep(1500 * time.Millisecond)
	}
}

func (b *Bot) ircsay(channel, message string) {
	b.ircSayQueue <- ircSayRequest{channel, message}
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

				b.say(p.BroadcasterId, p.Message)
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

// isPermanentChannel reports whether the bot must always remain in the given
// channel (case-insensitive login match): the creator's channel and the bot's
// own channel. These are joined at boot regardless of live state, never departed
// on go-offline, and can never be banished.
func (b *Bot) isPermanentChannel(chatterUserLogin string) bool {
	login := strings.ToLower(chatterUserLogin)
	return login == botCreatorChannel || login == strings.ToLower(b.config.BotUsername)
}

// isTrackedChannel reports whether login is currently tracked in config.Channels
// (an opted-in or !joinme'd channel). Used to gate go-live joins so a banished
// (opted-out, removed) channel is not rejoined when its lingering stream.online
// subscription fires.
func (b *Bot) isTrackedChannel(login string) bool {
	login = strings.ToLower(login)
	b.channelsMu.Lock()
	defer b.channelsMu.Unlock()
	for i := range b.config.Channels {
		c := b.config.Channels[i]
		if c.TwitchLogin != nil && *c.TwitchLogin == login {
			return true
		}
	}
	return false
}

// setLive records whether a channel (by lowercased login) is currently
// broadcasting. Called from the stream.online/offline webhooks and at boot.
func (b *Bot) setLive(login string, live bool) {
	login = strings.ToLower(login)
	b.liveMu.Lock()
	b.live[login] = live
	b.liveMu.Unlock()
}

// isChannelLive reports whether the bot should treat a channel as broadcasting
// for promo purposes. Permanent home channels always count as live so their
// promos (e.g. the bot's own self-promo) post regardless of stream state,
// preserving the pre-DB behavior.
func (b *Bot) isChannelLive(login string) bool {
	login = strings.ToLower(login)
	if b.isPermanentChannel(login) {
		return true
	}
	b.liveMu.Lock()
	defer b.liveMu.Unlock()
	return b.live[login]
}

// mentionsBot reports whether the message text @-mentions the bot by username.
func mentionsBot(message, botUsername string) bool {
	return strings.Contains(strings.ToLower(message), "@"+strings.ToLower(botUsername))
}

// isModOrBroadcaster reports whether the sender may run mod/broadcaster-gated
// commands in the channel the message came from.
func isModOrBroadcaster(message *chatMessageEvent) bool {
	if message.BroadcasterUserId == message.ChatterUserId {
		return true
	}

	for _, badge := range message.Badges {
		if badge.SetId == "moderator" {
			return true
		}
	}

	return false
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
