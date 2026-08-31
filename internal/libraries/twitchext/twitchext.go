// Package twitchext verifies the JWTs a Twitch Extension frontend hands to its
// backend (EBS): the per-viewer identity token from Twitch.ext.onAuthorized and
// the signed Bits transaction receipt from onTransactionComplete.
//
// Both are HS256 JWTs signed with the extension's shared secret. Twitch shows
// that secret base64-encoded in the developer console, so it must be
// base64-decoded before it can verify a signature -- signing against the
// encoded string is the classic Bits-verification bug.
package twitchext

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

// DecodeSecret turns the base64 secret from the developer console into the raw
// key bytes used for HMAC verification.
//
// Twitch's console gives the secret as base64 that is NOT "=" padded -- Node's
// Buffer.from(secret, "base64") is lenient, so Twitch's own EBS samples never
// pad, and the string it shows you commonly has length % 4 != 0. Go's
// base64.StdEncoding is strict and rejects that, so we normalise the padding and
// accept either the standard or URL-safe alphabet before decoding. Getting this
// wrong is what took down boot once: a real, valid secret was rejected.
func DecodeSecret(secretB64 string) ([]byte, error) {
	s := strings.TrimSpace(secretB64)
	if s == "" {
		return nil, fmt.Errorf("extension secret is empty")
	}
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	secret, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		// Fall back to the URL-safe alphabet (- and _ instead of + and /).
		secret, err = base64.URLEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("extension secret is not valid base64: %w", err)
		}
	}
	if len(secret) == 0 {
		return nil, fmt.Errorf("extension secret decoded to zero bytes")
	}
	return secret, nil
}

// IdentityClaims is the subset of the onAuthorized token we use. ChannelID is
// the broadcaster whose page the viewer is on -- the only trustworthy source of
// the channel, since a Bits receipt does not name one. UserID is present only
// when the viewer has shared identity; otherwise OpaqueUserID carries the
// anonymised id.
type IdentityClaims struct {
	ChannelID    string `json:"channel_id"`
	UserID       string `json:"user_id"`
	OpaqueUserID string `json:"opaque_user_id"`
	Role         string `json:"role"`
	jwtlib.RegisteredClaims
}

// ReceiptClaims is the Bits transaction receipt. Topic is
// "bits_transaction_receipt"; Data.Product.SKU and Data.TransactionID are what
// the relay needs (SKU to dispatch on, TransactionID as the idempotency key).
type ReceiptClaims struct {
	Topic string `json:"topic"`
	Data  struct {
		TransactionID string `json:"transactionId"`
		Time          string `json:"time"`
		UserID        string `json:"userId"`
		DomainID      string `json:"domainID"`
		Product       struct {
			SKU         string `json:"sku"`
			DisplayName string `json:"displayName"`
			Cost        struct {
				Amount int    `json:"amount"`
				Type   string `json:"type"`
			} `json:"cost"`
		} `json:"product"`
	} `json:"data"`
	jwtlib.RegisteredClaims
}

const receiptTopic = "bits_transaction_receipt"

// leeway tolerates small clock skew between Twitch and us on exp checks.
const leeway = 30 * time.Second

func verify(tokenString string, secret []byte, claims jwtlib.Claims) error {
	if strings.TrimSpace(tokenString) == "" {
		return fmt.Errorf("missing token")
	}
	token, err := jwtlib.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwtlib.Token) (any, error) {
			if t.Method != jwtlib.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method %q", t.Header["alg"])
			}
			return secret, nil
		},
		jwtlib.WithValidMethods([]string{jwtlib.SigningMethodHS256.Alg()}),
		jwtlib.WithExpirationRequired(),
		jwtlib.WithLeeway(leeway),
	)
	if err != nil || token == nil || !token.Valid {
		return fmt.Errorf("invalid token: %w", err)
	}
	return nil
}

// VerifyIdentity validates an onAuthorized token and returns its claims. The
// channel is required; a token without one cannot be routed.
func VerifyIdentity(tokenString string, secret []byte) (*IdentityClaims, error) {
	claims := &IdentityClaims{}
	if err := verify(tokenString, secret, claims); err != nil {
		return nil, err
	}
	if strings.TrimSpace(claims.ChannelID) == "" {
		return nil, fmt.Errorf("identity token carried no channel_id")
	}
	return claims, nil
}

// VerifyReceipt validates a Bits transaction receipt and returns its claims.
//
// Trust is the HS256 signature over THIS extension's secret: no other party
// holds it, so a valid signature already proves Twitch issued the receipt for
// this extension. We therefore authenticate on the signature and only
// sanity-check the payload shape (topic, sku, transaction id) -- we do NOT
// assert a domainID format, which varies and produced false 403s on the first
// live redemption.
func VerifyReceipt(tokenString string, secret []byte) (*ReceiptClaims, error) {
	claims := &ReceiptClaims{}
	if err := verify(tokenString, secret, claims); err != nil {
		return nil, err
	}
	if claims.Topic != receiptTopic {
		return nil, fmt.Errorf("receipt topic = %q, want %q", claims.Topic, receiptTopic)
	}
	if strings.TrimSpace(claims.Data.Product.SKU) == "" {
		return nil, fmt.Errorf("receipt carried no product sku")
	}
	if strings.TrimSpace(claims.Data.TransactionID) == "" {
		return nil, fmt.Errorf("receipt carried no transaction id")
	}
	return claims, nil
}
