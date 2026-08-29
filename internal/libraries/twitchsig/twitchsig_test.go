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
	h.Set(headerMessageID, testID)
	h.Set(headerMessageTimestamp, testTimestamp)
	h.Set(headerMessageSignature, sig)
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
