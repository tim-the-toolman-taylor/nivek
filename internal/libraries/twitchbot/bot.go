package twitchbot

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"slices"
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

type commandHandler func(b *Bot, message *twitch.PrivateMessage)

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

type BotUser struct {
	user.User
	hasPrivs bool
}

type Config struct {
	BotUsername     string
	BotId           string
	BotOAuth        string
	Channels        []BotUser // Changed from single Channel to multiple Channels
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

	// privNudgeMu guards privNudgeAt, the per-channel cooldown timestamps for the
	// "mod me" chat-read nudge (see chat_privs.go). Keyed by lowercased login.
	privNudgeMu sync.Mutex
	privNudgeAt map[string]time.Time

	// customMu guards customCommands, the per-channel custom ("channel"-scoped)
	// command sets. Outer key is the lowercased channel login (what handleMessage
	// dispatches on); inner key is the lowercased trigger. Loaded on stream.online
	// / boot-if-live, evicted on stream.offline (see custom_commands.go).
	customMu       sync.Mutex
	customCommands map[string]map[string]commands.Commands
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
		twitchClient:   twitchClient,
		overseerClient: overseerCli,
		tokenProvider:  config.TokenProvider,
		healInFlight:   make(map[string]bool),
		joinInFlight:   make(map[string]bool),
		commands:       cmds,

		dadUsage:       make(map[string]*dadStreamUsage),
		live:           make(map[string]bool),
		privNudgeAt:    make(map[string]time.Time),
		customCommands: make(map[string]map[string]commands.Commands),
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
				// This channel is already live at boot, so load its custom commands
				// now — a mid-stream restart won't get a fresh stream.online webhook.
				go b.loadCustomCommands(*channel.TwitchID, *channel.TwitchLogin)
			}
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
	// Normalize message
	msg := strings.TrimSpace(strings.ToLower(message.Message))
	chattername := message.User.Name

	// !newpromo takes free-form arguments (an interval + a message that may itself
	// contain other command words or URLs). Dispatch it up front and return so the
	// word-by-word scan below never treats the promo body as more commands.
	if msg == "!newpromo" || strings.HasPrefix(msg, "!newpromo ") {
		log.Printf("[CMD-RECV] [%s] %s: %q", message.Channel, chattername, msg)
		b.handleNewPromoCommand(&message)
		return
	}

	// Check for commands
	commandSeen := false
	for msgword := range strings.SplitSeq(msg, " ") {
		if handler, ok := b.commands[msgword]; ok {
			log.Printf("[CMD-RECV] [%s] %s: %q", message.Channel, chattername, msg)
			handler(b, &message)
			commandSeen = true
			continue
		}
		// Per-channel custom commands (loaded while the channel is live). A global
		// builtin with the same trigger wins — handled above via continue — so a
		// channel can't shadow a builtin in v1.
		if cmd, ok := b.customCommandFor(message.Channel, msgword); ok {
			commandSeen = true
			if cmd.ResponseTmpl != nil && meetsMinRole(&message, cmd.MinRole) {
				log.Printf("[CMD-RECV] [%s] %s: %q (custom)", message.Channel, chattername, msg)
				b.say(message.Channel, *cmd.ResponseTmpl)
			}
		}
	}

	// The bot is still serving this channel's commands over the legacy IRC
	// connection. If it hasn't yet been granted chat-read privileges (mod or
	// channel:bot), nudge the channel to mod the bot so it can migrate onto
	// Twitch's chat API. ensureChatReadPrivs is rate-limited and self-clearing:
	// it re-checks by attempting the subscription and stops nudging once granted.
	// Home channels are skipped (permanent, bot-controlled). Off the message path.
	if commandSeen && !b.isPermanentChannel(message.Channel) && !b.channelHasPrivs(message.Channel) {
		go b.ensureChatReadPrivs(message.Channel)
	}

	if _, ok := b.autoShout[message.Channel]; ok {
		if slices.Contains(
			b.autoShout[message.Channel],
			message.User.DisplayName,
		) {
			b.client.Say(message.Channel, fmt.Sprintf("!so @%s", chattername))
			log.Printf("[Auto Shout] given to %s in %s", chattername, message.Channel)
			// Persist the shout: bump shout_count and stamp this stream's key so
			// a restart mid-stream won't re-shout them. Off the message path.
			go b.incrementAutoShout(message.Channel, message.User.DisplayName)
			i := slices.Index(b.autoShout[message.Channel], message.User.DisplayName)
			b.autoShout[message.Channel] = slices.Delete(
				b.autoShout[message.Channel],
				i,
				i+1,
			)
		}
	}

	// DF commands are suspended until further notice
	// !DF takes arguments — handle separately from the exact-match commands below
	// if msg == "!df" || strings.HasPrefix(msg, "!df ") {
	// 	if message.Channel != botCreatorChannel {
	// 		return
	// 	}
	// 	args := strings.TrimSpace(strings.TrimPrefix(msg, "!df"))
	// 	b.handleDFCommand(message.Message, args, chattername, message.Channel)
	// 	return
	// }

	// if I want to manually insert a user into someone's channel, this will backfill the required information for webhooks
	if message.User.Name == message.Channel {
		go b.isLegacyChannel(&message)
	}
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

				b.say(p.Channelname, p.Message)
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

func (b *Bot) isLegacyChannel(message *twitch.PrivateMessage) {
	login := message.User.Name

	// Claim the heal under the lock: find a still-legacy row for this sender that
	// isn't already being healed, and snapshot it. Mutating a range-copy here
	// would be a no-op (it wouldn't touch config.Channels), so we index in.
	b.channelsMu.Lock()
	idx := -1
	for i := range b.config.Channels {
		if *b.config.Channels[i].TwitchLogin == login && b.config.Channels[i].TwitchID == nil {
			idx = i
			break
		}
	}
	if idx == -1 || b.healInFlight[login] {
		b.channelsMu.Unlock()
		return // not a pending legacy user, or a heal is already running for them
	}
	b.healInFlight[login] = true
	healed := b.config.Channels[idx] // snapshot under lock
	b.channelsMu.Unlock()

	healed.TwitchID = &message.User.ID
	healed.TwitchDisplayName = &message.User.DisplayName
	healed.TwitchLogin = &message.User.Name

	err := b.coreAPI.HealLegacyUser(&healed.User)
	if err != nil {
		log.Printf("failed to heal legacy user record: %+v - %s", healed, err.Error())
	} else {
		if _, subErr := b.twitchClient.SubscribeStreamOffline(context.Background(), *healed.TwitchID); subErr != nil {
			log.Printf("failed to subscribe stream.offline for healed user %s: %s", login, subErr.Error())
		}
		if _, subErr := b.twitchClient.SubscribeStreamOnline(context.Background(), *healed.TwitchID); subErr != nil {
			log.Printf("failed to subscribe stream.online for healed user %s: %s", login, subErr.Error())
		}
	}

	// Release the claim. On success, persist the patch into the in-memory list so
	// this user is never healed again this process; on failure, leave the row
	// legacy so a later message retries.
	b.channelsMu.Lock()
	delete(b.healInFlight, login)
	if err == nil && idx < len(b.config.Channels) &&
		*b.config.Channels[idx].TwitchLogin == login {
		b.config.Channels[idx].TwitchID = healed.TwitchID
		b.config.Channels[idx].TwitchDisplayName = healed.TwitchDisplayName
		b.config.Channels[idx].TwitchLogin = healed.TwitchLogin
	}
	b.channelsMu.Unlock()
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

// isPermanentChannel reports whether the bot must always remain in the given
// channel (case-insensitive login match): the creator's channel and the bot's
// own channel. These are joined at boot regardless of live state, never departed
// on go-offline, and can never be banished.
func (b *Bot) isPermanentChannel(login string) bool {
	login = strings.ToLower(login)
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
func isModOrBroadcaster(message *twitch.PrivateMessage) bool {
	return message.User.IsBroadcaster || message.User.IsMod
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
