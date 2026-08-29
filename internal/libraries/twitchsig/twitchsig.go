// Package twitchsig verifies the HMAC signature Twitch puts on every EventSub
// webhook delivery. It is a dependency-free leaf so any callback handler (the
// bot's webhook, the overlay relay's) can share one implementation instead of
// each carrying its own copy that could drift from Twitch's signing scheme.
package twitchsig

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// Twitch EventSub signature headers. Exported so callers that also parse these
// (a webhook reading the message id, or checking timestamp freshness) reference
// one definition rather than redeclaring the strings. http.Header.Get
// canonicalises the key, so these match regardless of the casing Twitch sends.
const (
	HeaderMessageID        = "Twitch-Eventsub-Message-Id"
	HeaderMessageTimestamp = "Twitch-Eventsub-Message-Timestamp"
	HeaderMessageSignature = "Twitch-Eventsub-Message-Signature"

	signaturePrefix = "sha256="
)

// Verify reports whether the request's signature header matches the HMAC-SHA256
// Twitch computes over messageID + timestamp + rawBody, keyed by the webhook
// secret. rawBody MUST be the exact bytes received: re-marshalling parsed JSON
// will not reproduce the signature. An empty secret always fails closed.
func Verify(header http.Header, rawBody []byte, secret string) bool {
	if secret == "" {
		return false
	}
	got := header.Get(HeaderMessageSignature)
	if !strings.HasPrefix(got, signaturePrefix) {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(header.Get(HeaderMessageID)))
	mac.Write([]byte(header.Get(HeaderMessageTimestamp)))
	mac.Write(rawBody)
	want := signaturePrefix + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(got), []byte(want))
}
