package twitchbot

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitcheventsub"
)

// privNudgeCooldown throttles the "mod me" nudge (and its subscription
// re-check) so it fires at most once per window per channel, not on every
// command.
const privNudgeCooldown = 15 * time.Minute

// chatReadSubType is the EventSub subscription that requires — and therefore
// proves — chat-read privileges: moderator status or the channel:bot scope.
const chatReadSubType = "channel.chat.message"

// channelHasPrivs reports whether the bot currently holds chat-read privileges
// for the channel with the given (lowercased) login, per its in-memory BotUser
// flag. Unknown channels report false.
func (b *Bot) channelHasPrivs(login string) bool {
	b.channelsMu.Lock()
	defer b.channelsMu.Unlock()
	for i := range b.config.Channels {
		if b.config.Channels[i].TwitchLogin != nil && *b.config.Channels[i].TwitchLogin == login {
			return b.config.Channels[i].hasPrivs
		}
	}
	return false
}

// setChannelPrivsByLogin flips hasPrivs for the channel matching login.
func (b *Bot) setChannelPrivsByLogin(login string, v bool) {
	b.channelsMu.Lock()
	defer b.channelsMu.Unlock()
	for i := range b.config.Channels {
		if b.config.Channels[i].TwitchLogin != nil && *b.config.Channels[i].TwitchLogin == login {
			b.config.Channels[i].hasPrivs = v
			return
		}
	}
}

// setChannelPrivsByID flips hasPrivs for the channel matching a broadcaster
// Twitch user id. Used by the revocation webhook, which carries the id (not the
// login).
func (b *Bot) setChannelPrivsByID(twitchID string, v bool) {
	b.channelsMu.Lock()
	defer b.channelsMu.Unlock()
	for i := range b.config.Channels {
		if b.config.Channels[i].TwitchID != nil && *b.config.Channels[i].TwitchID == twitchID {
			b.config.Channels[i].hasPrivs = v
			return
		}
	}
}

// hydrateChatReadPrivs seeds each channel's hasPrivs flag from Twitch's current
// EventSub state at boot: a broadcaster with an ENABLED channel.chat.message
// subscription has already granted chat-read (moderator or channel:bot). The
// subscription list on Twitch's side is the source of truth, so this restores
// the flags across restarts with no DB. Best-effort: a listing failure just
// leaves everyone un-granted and the per-command re-check recovers them.
func (b *Bot) hydrateChatReadPrivs(ctx context.Context) {
	subs, err := b.twitchClient.ListEventSubSubscriptions(ctx)
	if err != nil {
		log.Printf("chat-read privs: list subscriptions at boot failed: %s", err.Error())
		return
	}

	granted := make(map[string]bool)
	for _, s := range subs {
		if s.Type == chatReadSubType && s.Status == twitcheventsub.StatusEnabled {
			granted[s.Condition.BroadcasterUserID] = true
		}
	}

	b.channelsMu.Lock()
	n := 0
	for i := range b.config.Channels {
		if b.config.Channels[i].TwitchID != nil && granted[*b.config.Channels[i].TwitchID] {
			b.config.Channels[i].hasPrivs = true
			n++
		}
	}
	b.channelsMu.Unlock()
	log.Printf("chat-read privs: %d tracked channel(s) already granted at boot", n)
}

// ensureChatReadPrivs runs when a command is used in a channel that hasn't yet
// granted chat-read privileges. It re-checks by attempting the
// channel.chat.message subscription: if Twitch accepts it (or it already
// exists), the bot has been modded / granted channel:bot since boot, so we flip
// the flag and go quiet. Otherwise we post a nudge asking to be modded.
// Network-bound and rate-limited per channel; call off the message path.
func (b *Bot) ensureChatReadPrivs(login string) {
	if !b.claimPrivNudge(login) {
		return
	}

	twitchID := ""
	b.channelsMu.Lock()
	for i := range b.config.Channels {
		if b.config.Channels[i].TwitchLogin != nil && *b.config.Channels[i].TwitchLogin == login {
			if b.config.Channels[i].hasPrivs {
				b.channelsMu.Unlock()
				return // granted between dispatch and now
			}
			if b.config.Channels[i].TwitchID != nil {
				twitchID = *b.config.Channels[i].TwitchID
			}
			break
		}
	}
	b.channelsMu.Unlock()
	if twitchID == "" {
		return // legacy/un-healed row with no twitch id — nothing to subscribe with
	}

	res, err := b.twitchClient.SubscribeChannelChatMessages(context.Background(), twitchID)
	if err == nil && (res.OK() || res.AlreadyExists()) {
		b.setChannelPrivsByLogin(login, true)
		log.Printf("chat-read privs: granted for %s (subscribe status=%d)", login, res.StatusCode)
		return
	}
	if err != nil {
		log.Printf("chat-read privs: subscribe attempt for %s errored: %s", login, err.Error())
	}

	b.ircsay(login, fmt.Sprintf(
		"psst — mod me with /mod @%s so I can read chat through Twitch's API instead of this legacy connection 🙏",
		b.config.BotUsername,
	))
}

// claimPrivNudge returns true at most once per privNudgeCooldown per channel, so
// the nudge and its subscribe re-check don't fire on every command.
func (b *Bot) claimPrivNudge(login string) bool {
	b.privNudgeMu.Lock()
	defer b.privNudgeMu.Unlock()
	if last, ok := b.privNudgeAt[login]; ok && time.Since(last) < privNudgeCooldown {
		return false
	}
	b.privNudgeAt[login] = time.Now()
	return true
}
