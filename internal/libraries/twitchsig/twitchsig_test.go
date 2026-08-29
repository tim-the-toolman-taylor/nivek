package twitchsig

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
)

const (
	testSecret    = "s3cr3t"
	testID        = "msg-123"
	testTimestamp = "2026-08-29T12:00:00Z"
)

// sign builds the signature Twitch would send for the given parts.
func sign(secret, id, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(id))
	mac.Write([]byte(ts))
	mac.Write(body)
	return signaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

func headerFor(sig string) http.Header {
	h := http.Header{}
	h.Set(HeaderMessageID, testID)
	h.Set(HeaderMessageTimestamp, testTimestamp)
	h.Set(HeaderMessageSignature, sig)
	return h
}

func TestVerifyAcceptsGoodSignature(t *testing.T) {
	t.Parallel()
	body := []byte(`{"event":"cheer"}`)
	if !Verify(headerFor(sign(testSecret, testID, testTimestamp, body)), body, testSecret) {
		t.Fatal("valid signature rejected")
	}
}

func TestVerifyRejects(t *testing.T) {
	t.Parallel()
	body := []byte(`{"event":"cheer"}`)
	good := sign(testSecret, testID, testTimestamp, body)

	// A header whose id or timestamp differs from what was signed must fail:
	// both are part of the signed message, not just the body.
	swappedID := headerFor(good)
	swappedID.Set(HeaderMessageID, "other-id")
	swappedTS := headerFor(good)
	swappedTS.Set(HeaderMessageTimestamp, "2026-01-01T00:00:00Z")

	cases := []struct {
		name   string
		header http.Header
		body   []byte
		secret string
	}{
		{"tampered body", headerFor(good), []byte(`{"event":"redemption"}`), testSecret},
		{"wrong secret", headerFor(good), body, "other"},
		{"empty secret", headerFor(good), body, ""},
		{"missing prefix", headerFor(hex.EncodeToString([]byte("x"))), body, testSecret},
		{"empty signature", headerFor(""), body, testSecret},
		{"swapped message id", swappedID, body, testSecret},
		{"swapped timestamp", swappedTS, body, testSecret},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if Verify(tc.header, tc.body, tc.secret) {
				t.Fatalf("%s: expected rejection", tc.name)
			}
		})
	}
}
