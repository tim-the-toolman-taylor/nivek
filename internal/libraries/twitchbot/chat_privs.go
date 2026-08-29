package twitchbot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gempir/go-twitch-irc/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitcheventsub"
)

// chatReadSubAttemptCooldown bounds how often we POST channel.chat.message for
// one channel. The nudge itself is 1:1 with received IRC messages.
const chatReadSubAttemptCooldown = time.Minute

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
// leaves everyone un-granted and the per-message re-check recovers them.
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

func (b *Bot) chatReadNudgeText() string {
	return fmt.Sprintf(
		"psst — mod me with /mod @%s so I can read chat through Twitch's API instead of this legacy connection 🙏",
		b.config.BotUsername,
	)
}

func (b *Bot) isChatReadNudge(text string) bool {
	return strings.TrimSpace(text) == b.chatReadNudgeText()
}

func (b *Bot) isBotIRCUser(message twitch.PrivateMessage) bool {
	if id := strings.TrimSpace(b.config.BotId); id != "" && message.User.ID == id {
		return true
	}
	return strings.EqualFold(message.User.Name, b.config.BotUsername) ||
		strings.EqualFold(message.User.DisplayName, b.config.BotUsername)
}

func (b *Bot) enqueueChatReadNudge(login string) {
	select {
	case b.ircSayQueue <- ircSayRequest{login, b.chatReadNudgeText()}:
	default:
		log.Printf("[IRC] say queue full; dropping chat-read nudge for %s", login)
	}
}

// tryGrantChatRead re-checks channel.chat.message. If Twitch accepts it (or it
// already exists), the bot has been modded / granted channel:bot, so we flip
// the flag and stop nudging. Throttled per channel; does not send chat.
func (b *Bot) tryGrantChatRead(login string) {
	if !b.claimGrantAttempt(login) {
		return
	}
	defer b.clearGrantInFlight(login)

	twitchID := ""
	b.channelsMu.Lock()
	for i := range b.config.Channels {
		if b.config.Channels[i].TwitchLogin != nil && *b.config.Channels[i].TwitchLogin == login {
			if b.config.Channels[i].hasPrivs {
				b.channelsMu.Unlock()
				return
			}
			if b.config.Channels[i].TwitchID != nil {
				twitchID = *b.config.Channels[i].TwitchID
			}
			break
		}
	}
	b.channelsMu.Unlock()
	if twitchID == "" {
		return
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
}

func (b *Bot) claimGrantAttempt(login string) bool {
	b.grantMu.Lock()
	defer b.grantMu.Unlock()
	if b.grantInFlight[login] {
		return false
	}
	if last, ok := b.grantAt[login]; ok && time.Since(last) < chatReadSubAttemptCooldown {
		return false
	}
	b.grantInFlight[login] = true
	b.grantAt[login] = time.Now()
	return true
}

func (b *Bot) clearGrantInFlight(login string) {
	b.grantMu.Lock()
	delete(b.grantInFlight, login)
	b.grantMu.Unlock()
}
