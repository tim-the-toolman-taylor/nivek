package twitchbot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overseer"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overseer/wire"
)

// botCreatorChannel is the only Twitch channel from which !DF commands are
// accepted. Messages with the !df prefix from any other joined channel are
// dropped at dispatch — they never reach the executor on pad.

// dfWelcomeInterval is how often the bot posts the orientation message
// in dfCommandChannel. First tick fires one interval after bot start,
// not immediately — restarts shouldn't spam the channel.
const dfWelcomeInterval = 10 * time.Minute
const dfWelcomeMessage = `Welcome to the TWITCH PLAYS DWARF FORTRESS project! This is a work-in-progress. Please view the helpdoc at https://peanutbudderbot.com/df/help to learn how to play. I intend for https://peanutbudderbot.com/df to be used as your "dashboard" for viewing fortress information. Poke around and have fun!`

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

func (b *Bot) handleDFCommand(rawText, args, chatterTwitchLogin, broadcasterTwitchLogin, broadcasterId string) {
	action, err := overseer.ParseCommand(args)
	if err != nil {
		log.Printf("[DF] [%s] %s: parse failed for %q: %v", broadcasterTwitchLogin, chatterTwitchLogin, args, err)
		// Parse errors are silently rejected (locked design) — except a
		// RejectReason, which carries a chatter-safe "why" we do surface
		// (e.g. `appoint captain` → "needs a squad — not supported yet").
		var rr *overseer.RejectReason
		if errors.As(err, &rr) {
			resp := fmt.Sprintf("@%s — %s", chatterTwitchLogin, rr.Msg)
			b.say(&broadcasterId, &resp)
		}
		return
	}

	// help is a chat-response verb — no DFHack involvement, no executor
	// round-trip. Short-circuit here before the WS send.
	if action.Kind == wire.ActionKindHelp {
		resp := fmt.Sprintf(
			"@%s !DF: make [N] <material> <item> | place <item> <x> <y> <z> | brew [N] <fruit|plant> | mine <x,y,z> <x,y> | camera <x> <y> <z> | appoint <position> <id> | pause | unpause | help",
			chatterTwitchLogin,
		)
		b.say(&broadcasterId, &resp)
		log.Printf("[DF] [%s] %s: help requested", broadcasterTwitchLogin, chatterTwitchLogin)
		return
	}

	cmd := wire.Command{
		ID:         uuid.NewString(),
		ReceivedAt: time.Now().UTC(),
		RawText:    rawText,
		From: wire.CommandSource{
			Username: chatterTwitchLogin,
			Platform: wire.PlatformTwitch,
			Channel:  broadcasterTwitchLogin,
		},
		Action: action,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	executed, err := b.overseerClient.Send(ctx, cmd)
	if err != nil {
		log.Printf("[DF] [%s] %s: executor send failed: %v", broadcasterTwitchLogin, chatterTwitchLogin, err)
		resp := fmt.Sprintf("@%s — couldn't reach DF: %s", chatterTwitchLogin, err.Error())
		b.say(&broadcasterId, &resp)
		return
	}

	if executed.Result == wire.ExecResultError {
		log.Printf("[DF] [%s] %s: executor error: %s", broadcasterTwitchLogin, chatterTwitchLogin, executed.ErrorMessage)
		resp := fmt.Sprintf("@%s — couldn't queue: %s", chatterTwitchLogin, executed.ErrorMessage)
		b.say(&broadcasterId, &resp)
		return
	}

	resp := dfSuccessReply(chatterTwitchLogin, action)
	b.say(&broadcasterId, &resp)
}

func dfSuccessReply(chatterTwitchLogin string, action wire.Action) string {
	switch action.Kind {
	case wire.ActionKindManufacture:
		mat := ""
		if action.Material != nil {
			mat = *action.Material + " "
		}
		return fmt.Sprintf("@%s queued %d %s%s%s", chatterTwitchLogin, action.Quantity, mat, action.Item, pluralize(action.Quantity))
	case wire.ActionKindPause:
		return fmt.Sprintf("@%s paused DF", chatterTwitchLogin)
	case wire.ActionKindUnpause:
		return fmt.Sprintf("@%s unpaused DF", chatterTwitchLogin)
	case wire.ActionKindCamera:
		if action.Position != nil {
			return fmt.Sprintf("@%s moved camera to (%d, %d, %d)", chatterTwitchLogin, action.Position.X, action.Position.Y, action.Position.Z)
		}
		return fmt.Sprintf("@%s moved camera", chatterTwitchLogin)
	case wire.ActionKindPlace:
		if action.Position != nil {
			return fmt.Sprintf("@%s placed %s at (%d, %d, %d)", chatterTwitchLogin, action.Item, action.Position.X, action.Position.Y, action.Position.Z)
		}
		return fmt.Sprintf("@%s placed %s", chatterTwitchLogin, action.Item)
	case wire.ActionKindBrew:
		return fmt.Sprintf("@%s queued %d brew%s from %s", chatterTwitchLogin, action.Quantity, pluralize(action.Quantity), action.Item)
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
				chatterTwitchLogin, dx, dy, noun,
				action.Region.Min.X, action.Region.Min.Y, action.Region.Min.Z,
				action.Region.Max.X, action.Region.Max.Y,
			)
		}
		return fmt.Sprintf("@%s designated %s area", chatterTwitchLogin, noun)
	case wire.ActionKindStockpile:
		if action.Region != nil {
			dx := abs(action.Region.Max.X-action.Region.Min.X) + 1
			dy := abs(action.Region.Max.Y-action.Region.Min.Y) + 1
			return fmt.Sprintf("@%s built %dx%d %s stockpile at (%d, %d, %d)",
				chatterTwitchLogin, dx, dy, action.Item,
				action.Region.Min.X, action.Region.Min.Y, action.Region.Min.Z)
		}
		return fmt.Sprintf("@%s built %s stockpile", chatterTwitchLogin, action.Item)
	case wire.ActionKindZone:
		if action.Region != nil {
			dx := abs(action.Region.Max.X-action.Region.Min.X) + 1
			dy := abs(action.Region.Max.Y-action.Region.Min.Y) + 1
			return fmt.Sprintf("@%s designated %dx%d %s zone at (%d, %d, %d)",
				chatterTwitchLogin, dx, dy, action.Item,
				action.Region.Min.X, action.Region.Min.Y, action.Region.Min.Z)
		}
		return fmt.Sprintf("@%s designated %s zone", chatterTwitchLogin, action.Item)
	case wire.ActionKindAppoint:
		return fmt.Sprintf("@%s appointed unit #%d as %s", chatterTwitchLogin, action.UnitID, action.Office)
	case wire.ActionKindTaskat:
		mat := ""
		if action.Material != nil {
			mat = *action.Material + " "
		}
		return fmt.Sprintf("@%s queued %d %s%s%s at workshop #%d",
			chatterTwitchLogin, action.Quantity, mat, action.Item, pluralize(action.Quantity), action.WorkshopID)
	default:
		return fmt.Sprintf("@%s executed %s", chatterTwitchLogin, action.Kind)
	}
}
