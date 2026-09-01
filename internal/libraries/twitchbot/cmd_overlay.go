package twitchbot

import (
	"log"
	"strings"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
)

const (
	// maxOverlayArgs and maxOverlayArgLen bound what a chatter can push through
	// to someone's machine. The overlay's own commands take at most one
	// argument; this is slack, not a target.
	maxOverlayArgs   = 4
	maxOverlayArgLen = 32
)

// overlayAction builds the handler for one overlay command. The action is the
// stable name the overlay dispatches on -- the handler_key minus its "overlay_"
// prefix -- so several triggers can share one action (the aliases !g,
// !grenade, !grenades, !granade and !grandma all resolve to "grenade").
func overlayAction(action string) commandHandler {
	return func(b *Bot, message *chatMessageEvent) {
		b.dispatchOverlayCommand(action, message)
	}
}

// dispatchOverlayCommand forwards a matched command to the broadcaster's
// overlay. It runs off the message path: core-api writes to the event log and
// pushes to the socket, and a slow round trip there must not hold up the rest
// of chat.
//
// Nothing is said in chat either way. A command that reached the overlay speaks
// for itself on stream, and one that did not is not worth narrating -- the
// overlay being closed is the streamer's business, not chat's.
func (b *Bot) dispatchOverlayCommand(action string, message *chatMessageEvent) {
	if message.MessageId == "" {
		// The dedupe key is built from the message id; without one the relay
		// cannot tell a retry from a second command.
		log.Printf("[OVERLAY-CMD] [%s] %s: no message id, dropping", message.BroadcasterUserLogin, action)
		return
	}

	req := api.OverlayDispatch{
		BroadcasterUserID: message.BroadcasterUserId,
		DedupeKey:         message.MessageId + ":" + action,
		Action:            action,
		Args:              overlayArgs(message.Message.Text, message.matchedTrigger),
		UserID:            message.ChatterUserId,
		UserLogin:         message.ChatterUserLogin,
		UserName:          message.ChatterUserName,
	}

	go func() {
		delivered, err := b.coreAPI.DispatchOverlayCommand(req)
		if err != nil {
			log.Printf("[OVERLAY-CMD] [%s] %s: %s", message.BroadcasterUserLogin, action, err.Error())
			return
		}
		if !delivered {
			log.Printf("[OVERLAY-CMD] [%s] %s from %s: no overlay attached, dropped",
				message.BroadcasterUserLogin, action, message.ChatterUserLogin)
			return
		}
		log.Printf("[OVERLAY-CMD] [%s] %s from %s", message.BroadcasterUserLogin, action, message.ChatterUserLogin)
	}()
}

// overlayArgs returns the words following the matched trigger, in the original
// casing, stopping at the next command word so "!b 20 !nuke" gives !b the
// argument "20" and nothing else.
func overlayArgs(text, trigger string) []string {
	if trigger == "" {
		return nil
	}

	fields := strings.Fields(text)
	start := -1
	for i, f := range fields {
		if strings.EqualFold(f, trigger) {
			start = i + 1
			break
		}
	}
	if start < 0 || start >= len(fields) {
		return nil
	}

	var args []string
	for _, f := range fields[start:] {
		if strings.HasPrefix(f, "!") {
			break
		}
		if len(f) > maxOverlayArgLen {
			f = f[:maxOverlayArgLen]
		}
		args = append(args, f)
		if len(args) == maxOverlayArgs {
			break
		}
	}
	return args
}

// setCapabilities replaces a channel's capability set. Safe to call repeatedly;
// an empty set drops the channel from the map entirely so "absent" and "holds
// nothing" stay the same thing.
func (b *Bot) setCapabilities(login string, caps []string) {
	key := strings.ToLower(login)
	b.capMu.Lock()
	defer b.capMu.Unlock()

	if len(caps) == 0 {
		delete(b.capabilities, key)
		return
	}
	set := make(map[string]bool, len(caps))
	for _, c := range caps {
		set[c] = true
	}
	b.capabilities[key] = set
}

// dropCapabilities evicts a channel's capability set, on stream.offline.
func (b *Bot) dropCapabilities(login string) {
	key := strings.ToLower(login)
	b.capMu.Lock()
	delete(b.capabilities, key)
	b.capMu.Unlock()
}

// channelHas reports whether a channel holds a capability. A channel that has
// not been loaded holds nothing: a gated command stays off rather than firing
// where it cannot work.
func (b *Bot) channelHas(login, capability string) bool {
	b.capMu.Lock()
	defer b.capMu.Unlock()
	return b.capabilities[strings.ToLower(login)][capability]
}

// commandAllowedIn reports whether a global trigger may dispatch in a channel.
// Ungated triggers always may; a gated one needs the channel to hold its
// capability.
func (b *Bot) commandAllowedIn(login, trigger string) bool {
	required, gated := b.commandRequires[trigger]
	if !gated {
		return true
	}
	return b.channelHas(login, required)
}
