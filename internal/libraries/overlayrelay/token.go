package overlayrelay

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// TokenPrefix makes a leaked token greppable in logs and paste sites, and lets
// us reject obvious non-tokens before touching the database.
const TokenPrefix = "rsov_"

const tokenBytes = 32

// MintToken returns a fresh device token and the hash to persist. The plaintext
// is shown to the streamer once and never stored: authentication re-hashes what
// the client presents and compares against token_hash.
//
// This is deliberately an opaque random token rather than a JWT. Device tokens
// are long-lived, and a JWT cannot be revoked without a blocklist that reduces
// it to a database lookup anyway -- so we do the lookup and get instant
// revocation from the dashboard for free. The short-lived browser session keeps
// using the jwt package.
func MintToken() (token string, hash string, err error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate device token: %w", err)
	}
	token = TokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	return token, HashToken(token), nil
}

// HashToken is sha256-hex of the token. A device token is 256 bits of CSPRNG
// output, not a password: it has no guessable structure to brute force, so a
// plain cryptographic hash is the right primitive and a KDF would only add
// latency to every frame-zero handshake.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// LooksLikeToken screens malformed input before a database round trip.
func LooksLikeToken(token string) bool {
	if !strings.HasPrefix(token, TokenPrefix) {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(token, TokenPrefix))
	return err == nil && len(raw) == tokenBytes
}
