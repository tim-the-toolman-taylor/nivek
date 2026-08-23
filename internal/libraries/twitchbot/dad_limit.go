package twitchbot

import (
	"log"
	"math/rand/v2"
	"strconv"
	"strings"
)

// dadPerStreamLimit is how many !dad rolls a single chatter gets per stream
// before the bot starts turning them away. One-line tunable.
const dadPerStreamLimit = 5

// dadRejectResponses are the dad-flavored "you're capped" lines. One is sent when
// a chatter first crosses the limit in a stream; after that the bot stays silent
// for them until the next stream. Hardcoded (not DB-backed) on purpose: the
// over-limit path stays fully in-process — no round-trip, and no way for a
// spammer to turn rejections into database load.
var dadRejectResponses = []string{
	"That's enough dad jokes for one stream, kiddo — go play outside.",
	"I'm all out of material for you. Ask your old man again next stream.",
	"That's it, I'm going to the bar. Be back next stream.",
}

// dadStreamUsage tracks, for one channel's current stream, how many times each
// chatter has rolled !dad. streamKey is the stream.online started_at, kept for
// logging; correctness comes from the go-live reset / go-offline evict lifecycle,
// not from comparing keys.
type dadStreamUsage struct {
	streamKey string
	counts    map[string]int // chatter login -> rolls served this stream
}

// dadDecision is the outcome of a rate-limit check for one !dad roll.
type dadDecision int

const (
	dadAllow  dadDecision = iota // under the limit: serve a real response
	dadReject                    // just crossed the limit: send one reject line
	dadSilent                    // already warned this stream: say nothing
)

// checkDadLimit records a !dad roll for chatter in channel and returns whether to
// serve a response, send the one-time reject, or stay silent. Both names are
// lowercased Twitch logins. If no stream entry exists yet (bot restarted
// mid-stream, or the go-live webhook was missed) one is created lazily; a later
// go-live resets it.
func (b *Bot) checkDadLimit(broadcasterUserLogin, chatter string) dadDecision {
	chatter = strings.ToLower(chatter)

	b.dadMu.Lock()
	defer b.dadMu.Unlock()

	u, ok := b.dadUsage[broadcasterUserLogin]
	if !ok {
		u = &dadStreamUsage{counts: make(map[string]int)}
		b.dadUsage[broadcasterUserLogin] = u
	}

	switch n := u.counts[chatter]; {
	case n < dadPerStreamLimit:
		u.counts[chatter] = n + 1
		return dadAllow
	case n == dadPerStreamLimit:
		u.counts[chatter] = n + 1 // advance past the limit so we warn exactly once
		return dadReject
	default:
		return dadSilent
	}
}

// randomDadReject returns a random hardcoded reject line.
func randomDadReject() string {
	return dadRejectResponses[rand.IntN(len(dadRejectResponses))]
}

// startDadStream (re)sets a channel's per-stream !dad counters. Called on
// stream.online so every stream starts each chatter fresh.
func (b *Bot) startDadStream(channel, streamKey string) {
	channel = strings.ToLower(channel)
	b.dadMu.Lock()
	b.dadUsage[channel] = &dadStreamUsage{streamKey: streamKey, counts: make(map[string]int)}
	b.dadMu.Unlock()
	log.Printf("[DAD] [%s] stream started (%s) — usage counters reset", channel, streamKey)
}

// endDadStream drops a channel's per-stream !dad counters. Called on
// stream.offline; this event-driven eviction is the primary cleanup — there is no
// scheduled job. A missed offline just lingers until the next go-live resets it.
func (b *Bot) endDadStream(channel string) {
	channel = strings.ToLower(channel)
	b.dadMu.Lock()
	delete(b.dadUsage, channel)
	b.dadMu.Unlock()
}

// rehydrateDadUsage seeds a live channel's in-memory !dad counters from the
// persisted dad_usage table on boot, so a restart mid-stream doesn't hand every
// chatter a fresh allotment. core-api returns only rows stamped with the
// broadcaster's current stream_key (a stale stream's rows are ignored, i.e. a new
// stream still starts fresh). Merges by max rather than overwriting: a message
// that lazily created a counter between Join and this call keeps its higher value.
func (b *Bot) rehydrateDadUsage(login, broadcasterID string) {
	id, err := strconv.Atoi(broadcasterID)
	if err != nil {
		log.Printf("[DAD] invalid twitch_id %q for channel %s; skipping usage rehydrate: %v", broadcasterID, login, err)
		return
	}
	usage, err := b.coreAPI.GetDadUsage(id)
	if err != nil {
		log.Printf("[DAD] failed to rehydrate usage for %s: %v", login, err)
		return
	}
	if len(usage) == 0 {
		return
	}

	channel := strings.ToLower(login)
	b.dadMu.Lock()
	u, ok := b.dadUsage[channel]
	if !ok {
		u = &dadStreamUsage{counts: make(map[string]int)}
		b.dadUsage[channel] = u
	}
	for chatter, count := range usage {
		if count > u.counts[chatter] {
			u.counts[chatter] = count
		}
	}
	b.dadMu.Unlock()
	log.Printf("[DAD] [%s] rehydrated %d chatter(s) from persisted usage", channel, len(usage))
}

// persistDadRoll writes one counted !dad roll through to dad_usage so it survives
// a restart. Best-effort, meant to run in a goroutine: it resolves the
// broadcaster's Twitch id from the tracked channel list and stamps the roll with
// the current stream_key server-side. Only called for allow/reject rolls (n <=
// limit); over-limit rolls stay silent and never touch the DB, so a spammer can't
// turn rejections into write load. The chatter key is lowercased to match the
// in-memory counter's key (see checkDadLimit).
func (b *Bot) persistDadRoll(channelLogin, chattername string) {
	tid, ok := b.channelTwitchID(channelLogin)
	if !ok {
		// Legacy/untracked channel with no twitch_id: nothing to persist against.
		return
	}
	id, err := strconv.Atoi(tid)
	if err != nil {
		log.Printf("[DAD] invalid twitch_id %q for channel %s: %v", tid, channelLogin, err)
		return
	}
	if err := b.coreAPI.IncrementDadRoll(id, strings.ToLower(chattername)); err != nil {
		log.Printf("[DAD] failed to persist roll for %s in %s: %v", chattername, channelLogin, err)
	}
}
