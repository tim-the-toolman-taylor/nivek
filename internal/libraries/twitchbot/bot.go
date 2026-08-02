package twitchbot

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/gempir/go-twitch-irc/v4"
	"github.com/google/uuid"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overseer"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overseer/wire"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

// dfCommandChannel is the only Twitch channel from which !DF commands are
// accepted. Messages with the !df prefix from any other joined channel are
// dropped at dispatch — they never reach the executor on pad.
const dfCommandChannel = "timallenfanclubofficial"

// dfWelcomeInterval is how often the bot posts the orientation message
// in dfCommandChannel. First tick fires one interval after bot start,
// not immediately — restarts shouldn't spam the channel.
const dfWelcomeInterval = 10 * time.Minute

const dfWelcomeMessage = `Welcome to the TWITCH PLAYS DWARF FORTRESS project! This is a work-in-progress. Please view the helpdoc at https://peanutbudderbot.com/df/help to learn how to play. I intend for https://peanutbudderbot.com/df to be used as your "dashboard" for viewing fortress information. Poke around and have fun!`

type commandHandler func(b *Bot, chattername, channel string)

type Config struct {
	BotUsername     string
	BotOAuth        string
	Channels        map[string]user.User // Changed from single Channel to multiple Channels
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
	overseerClient *overseer.Client
	sayQueue       chan sayRequest
}

func NewBot(coreAPI *api.CoreAPIClient, config Config) (*Bot, error) {

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
	// @TODO::refactor this to use the core-api state management system to join channels at startup, then wire up a way to react to incoming webhooks
	// Join all channels
	for _, channel := range b.config.Channels {
		if channel.TwitchLogin != nil && channel.IsLive {
			b.client.Join(*channel.TwitchLogin)
			log.Printf("Joining channel: %+v", channel)
		}
	}

	// Start reset timer
	go b.counters.StartResetTimer(ctx)

	// setup webhook listener
	NewWebhookListener(b)

	// Start the DF welcome/orientation announcer in dfCommandChannel.
	// go b.runDFWelcomeLoop(ctx) // temporarily disabled while bot is migrated from Pi -> VPS

	// Start IRC client with panic recovery and auto-reconnect
	go func() {
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
	}()

	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

func (b *Bot) Stop() {
	log.Println("Disconnecting from Twitch...")
	b.client.Disconnect()

	// Save counters one last time
	if err := b.counters.Save(); err != nil {
		log.Printf("Error saving counters on shutdown: %v", err)
	}
}

func (b *Bot) handleMessage(message twitch.PrivateMessage) {
	// Normalize message
	msg := strings.TrimSpace(strings.ToLower(message.Message))
	chattername := message.User.Name
	channel := message.Channel

	var commands = map[string]commandHandler{
		"!bread": (*Bot).handleBreadCommand,
		"!fish":  (*Bot).handleFishCommand,
		"!dad":   func(b *Bot, _, channel string) { b.say(channel, "still out getting milk!") },
		"!lurk":  (*Bot).handleLurkCommand,
	}

	if slices.Contains(slices.Collect(maps.Keys(commands)), msg) {
		log.Printf("[CMD-RECV] [%s] %s: %q", channel, chattername, msg)
	}

	// if b.autoShout.OnMessage(channel, chattername) {
	// 	b.client.Say(channel, fmt.Sprintf("!so @%s", chattername))
	// }

	// !DF takes arguments — handle separately from the exact-match commands below
	if msg == "!df" || strings.HasPrefix(msg, "!df ") {
		if channel != dfCommandChannel {
			return
		}
		args := strings.TrimSpace(strings.TrimPrefix(msg, "!df"))
		b.handleDFCommand(message.Message, args, chattername, channel)
		return
	}

	// Check for commands
	for cmd, handler := range commands {
		if strings.Contains(msg, cmd) {
			handler(b, chattername, channel)
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

// runDFWelcomeLoop posts dfWelcomeMessage to dfCommandChannel on
// dfWelcomeInterval ticks. Ticker waits one interval before the first
// tick, which is the desired behavior — bot restarts don't re-announce.
func (b *Bot) runDFWelcomeLoop(ctx context.Context) {
	ticker := time.NewTicker(dfWelcomeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			//			b.say(dfCommandChannel, dfWelcomeMessage)
		}
	}
}

func (b *Bot) handleDFCommand(rawText, args, username, channel string) {
	action, err := overseer.ParseCommand(args)
	if err != nil {
		log.Printf("[DF] [%s] %s: parse failed for %q: %v", channel, username, args, err)
		// Parse errors are silently rejected (locked design) — except a
		// RejectReason, which carries a chatter-safe "why" we do surface
		// (e.g. `appoint captain` → "needs a squad — not supported yet").
		var rr *overseer.RejectReason
		if errors.As(err, &rr) {
			b.say(channel, fmt.Sprintf("@%s — %s", username, rr.Msg))
		}
		return
	}

	// help is a chat-response verb — no DFHack involvement, no executor
	// round-trip. Short-circuit here before the WS send.
	if action.Kind == wire.ActionKindHelp {
		b.say(channel, fmt.Sprintf(
			"@%s !DF: make [N] <material> <item> | place <item> <x> <y> <z> | brew [N] <fruit|plant> | mine <x,y,z> <x,y> | camera <x> <y> <z> | appoint <position> <id> | pause | unpause | help",
			username,
		))
		log.Printf("[DF] [%s] %s: help requested", channel, username)
		return
	}

	cmd := wire.Command{
		ID:         uuid.NewString(),
		ReceivedAt: time.Now().UTC(),
		RawText:    rawText,
		From: wire.CommandSource{
			Username: username,
			Platform: wire.PlatformTwitch,
			Channel:  channel,
		},
		Action: action,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	executed, err := b.overseerClient.Send(ctx, cmd)
	if err != nil {
		log.Printf("[DF] [%s] %s: executor send failed: %v", channel, username, err)
		b.say(channel, fmt.Sprintf("@%s — couldn't reach DF: %s", username, err.Error()))
		return
	}

	if executed.Result == wire.ExecResultError {
		log.Printf("[DF] [%s] %s: executor error: %s", channel, username, executed.ErrorMessage)
		b.say(channel, fmt.Sprintf("@%s — couldn't queue: %s", username, executed.ErrorMessage))
		return
	}

	b.say(channel, dfSuccessReply(username, action))
}

func dfSuccessReply(username string, action wire.Action) string {
	switch action.Kind {
	case wire.ActionKindManufacture:
		mat := ""
		if action.Material != nil {
			mat = *action.Material + " "
		}
		return fmt.Sprintf("@%s queued %d %s%s%s", username, action.Quantity, mat, action.Item, pluralize(action.Quantity))
	case wire.ActionKindPause:
		return fmt.Sprintf("@%s paused DF", username)
	case wire.ActionKindUnpause:
		return fmt.Sprintf("@%s unpaused DF", username)
	case wire.ActionKindCamera:
		if action.Position != nil {
			return fmt.Sprintf("@%s moved camera to (%d, %d, %d)", username, action.Position.X, action.Position.Y, action.Position.Z)
		}
		return fmt.Sprintf("@%s moved camera", username)
	case wire.ActionKindPlace:
		if action.Position != nil {
			return fmt.Sprintf("@%s placed %s at (%d, %d, %d)", username, action.Item, action.Position.X, action.Position.Y, action.Position.Z)
		}
		return fmt.Sprintf("@%s placed %s", username, action.Item)
	case wire.ActionKindBrew:
		return fmt.Sprintf("@%s queued %d brew%s from %s", username, action.Quantity, pluralize(action.Quantity), action.Item)
	case wire.ActionKindMine, wire.ActionKindChannel, wire.ActionKindDigRamp, wire.ActionKindCutTree:
		noun := "dig"
		switch action.Kind {
		case wire.ActionKindChannel:
			noun = "channel"
		case wire.ActionKindDigRamp:
			noun = "ramp"
		case wire.ActionKindCutTree:
			noun = "tree-chop"
		}
		if action.Region != nil {
			dx := abs(action.Region.Max.X-action.Region.Min.X) + 1
			dy := abs(action.Region.Max.Y-action.Region.Min.Y) + 1
			return fmt.Sprintf("@%s designated %dx%d %s area from (%d, %d, %d) to (%d, %d)",
				username, dx, dy, noun,
				action.Region.Min.X, action.Region.Min.Y, action.Region.Min.Z,
				action.Region.Max.X, action.Region.Max.Y,
			)
		}
		return fmt.Sprintf("@%s designated %s area", username, noun)
	case wire.ActionKindStockpile:
		if action.Region != nil {
			dx := abs(action.Region.Max.X-action.Region.Min.X) + 1
			dy := abs(action.Region.Max.Y-action.Region.Min.Y) + 1
			return fmt.Sprintf("@%s built %dx%d %s stockpile at (%d, %d, %d)",
				username, dx, dy, action.Item,
				action.Region.Min.X, action.Region.Min.Y, action.Region.Min.Z)
		}
		return fmt.Sprintf("@%s built %s stockpile", username, action.Item)
	case wire.ActionKindZone:
		if action.Region != nil {
			dx := abs(action.Region.Max.X-action.Region.Min.X) + 1
			dy := abs(action.Region.Max.Y-action.Region.Min.Y) + 1
			return fmt.Sprintf("@%s designated %dx%d %s zone at (%d, %d, %d)",
				username, dx, dy, action.Item,
				action.Region.Min.X, action.Region.Min.Y, action.Region.Min.Z)
		}
		return fmt.Sprintf("@%s designated %s zone", username, action.Item)
	case wire.ActionKindAppoint:
		return fmt.Sprintf("@%s appointed unit #%d as %s", username, action.UnitID, action.Office)
	case wire.ActionKindTaskat:
		mat := ""
		if action.Material != nil {
			mat = *action.Material + " "
		}
		return fmt.Sprintf("@%s queued %d %s%s%s at workshop #%d",
			username, action.Quantity, mat, action.Item, pluralize(action.Quantity), action.WorkshopID)
	default:
		return fmt.Sprintf("@%s executed %s", username, action.Kind)
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
