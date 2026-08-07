package twitchbot

import (
	"log"
	"math/rand/v2"
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
func (b *Bot) checkDadLimit(channel, chatter string) dadDecision {
	channel = strings.ToLower(channel)
	chatter = strings.ToLower(chatter)

	b.dadMu.Lock()
	defer b.dadMu.Unlock()

	u, ok := b.dadUsage[channel]
	if !ok {
		u = &dadStreamUsage{counts: make(map[string]int)}
		b.dadUsage[channel] = u
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
