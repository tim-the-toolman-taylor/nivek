package overlayrelay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"
)

const testSecret = "s3cret-webhook-value"

func signedHeader(messageID, timestamp, body, secret string) http.Header {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(messageID))
	mac.Write([]byte(timestamp))
	mac.Write([]byte(body))

	h := http.Header{}
	h.Set(HeaderMessageID, messageID)
	h.Set(HeaderMessageTimestamp, timestamp)
	h.Set(HeaderMessageSignature, "sha256="+hex.EncodeToString(mac.Sum(nil)))
	return h
}

func TestVerifySignatureAcceptsGenuineMessage(t *testing.T) {
	body := `{"subscription":{"type":"channel.cheer"}}`
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	header := signedHeader("msg-1", ts, body, testSecret)

	if !VerifySignature(header, []byte(body), testSecret) {
		t.Fatal("genuine message rejected")
	}
}

func TestVerifySignatureRejectsTampering(t *testing.T) {
	body := `{"subscription":{"type":"channel.cheer"}}`
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	header := signedHeader("msg-1", ts, body, testSecret)

	cases := map[string]func() bool{
		"altered body": func() bool {
			return VerifySignature(header, []byte(body+" "), testSecret)
		},
		"wrong secret": func() bool {
			return VerifySignature(header, []byte(body), "not-the-secret")
		},
		"empty secret": func() bool {
			return VerifySignature(header, []byte(body), "")
		},
		"swapped message id": func() bool {
			swapped := header.Clone()
			swapped.Set(HeaderMessageID, "msg-2")
			return VerifySignature(swapped, []byte(body), testSecret)
		},
		"swapped timestamp": func() bool {
			swapped := header.Clone()
			swapped.Set(HeaderMessageTimestamp, time.Now().Add(time.Hour).Format(time.RFC3339Nano))
			return VerifySignature(swapped, []byte(body), testSecret)
		},
		"missing signature": func() bool {
			bare := header.Clone()
			bare.Del(HeaderMessageSignature)
			return VerifySignature(bare, []byte(body), testSecret)
		},
		"unprefixed signature": func() bool {
			raw := header.Clone()
			raw.Set(HeaderMessageSignature, "deadbeef")
			return VerifySignature(raw, []byte(body), testSecret)
		},
	}

	for name, accepted := range cases {
		if accepted() {
			t.Errorf("%s: signature accepted, want rejected", name)
		}
	}
}

func TestIsStale(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		timestamp string
		want      bool
	}{
		{"fresh", now.Format(time.RFC3339Nano), false},
		{"just inside window", now.Add(-9 * time.Minute).Format(time.RFC3339Nano), false},
		{"replayed", now.Add(-30 * time.Minute).Format(time.RFC3339Nano), true},
		{"future skew beyond window", now.Add(30 * time.Minute).Format(time.RFC3339Nano), true},
		{"unparseable", "not-a-timestamp", true},
		{"absent", "", true},
	}

	for _, tc := range tests {
		h := http.Header{}
		if tc.timestamp != "" {
			h.Set(HeaderMessageTimestamp, tc.timestamp)
		}
		if got := IsStale(h, now, MaxMessageAge); got != tc.want {
			t.Errorf("%s: IsStale = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestParseNotificationCheer(t *testing.T) {
	body := `{
		"subscription": {"type":"channel.cheer","condition":{"broadcaster_user_id":"443367221"}},
		"event": {"user_id":"1234","user_login":"cheerer","user_name":"Cheerer",
		          "is_anonymous":false,"bits":500,"message":"Cheer500 nuke"}
	}`
	header := http.Header{}
	header.Set(HeaderMessageID, "msg-cheer")

	in, err := ParseNotification(header, []byte(body))
	if err != nil {
		t.Fatalf("ParseNotification: %v", err)
	}
	if in.Kind != KindCheer {
		t.Fatalf("kind = %q, want %q", in.Kind, KindCheer)
	}
	if in.BroadcasterUserID != "443367221" {
		t.Fatalf("broadcaster = %q", in.BroadcasterUserID)
	}
	if in.TwitchMessageID != "msg-cheer" {
		t.Fatalf("message id = %q", in.TwitchMessageID)
	}

	var payload CheerPayload
	if err := json.Unmarshal(in.Payload, &payload); err != nil {
		t.Fatalf("payload did not round trip: %v", err)
	}
	if payload.Bits != 500 || payload.Message != "Cheer500 nuke" || payload.UserLogin != "cheerer" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestParseNotificationAnonymousCheer(t *testing.T) {
	// Anonymous cheers carry no identity at all. The overlay must still be able
	// to act on them, so parsing has to survive the missing fields.
	body := `{
		"subscription": {"type":"channel.cheer","condition":{"broadcaster_user_id":"443367221"}},
		"event": {"is_anonymous":true,"bits":100,"message":"Cheer100"}
	}`
	header := http.Header{}
	header.Set(HeaderMessageID, "msg-anon")

	in, err := ParseNotification(header, []byte(body))
	if err != nil {
		t.Fatalf("ParseNotification: %v", err)
	}

	var payload CheerPayload
	if err := json.Unmarshal(in.Payload, &payload); err != nil {
		t.Fatalf("payload did not round trip: %v", err)
	}
	if !payload.IsAnonymous || payload.Bits != 100 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.UserLogin != "" || payload.UserID != "" {
		t.Fatalf("anonymous cheer carried identity: %+v", payload)
	}
}

func TestParseNotificationRedemption(t *testing.T) {
	body := `{
		"subscription": {"type":"channel.channel_points_custom_reward_redemption.add",
		                 "condition":{"broadcaster_user_id":"443367221"}},
		"event": {"user_id":"99","user_login":"viewer","user_name":"Viewer",
		          "user_input":"go on then","status":"unfulfilled",
		          "reward":{"id":"r1","title":"Granades!","cost":250}}
	}`
	header := http.Header{}
	header.Set(HeaderMessageID, "msg-redeem")

	in, err := ParseNotification(header, []byte(body))
	if err != nil {
		t.Fatalf("ParseNotification: %v", err)
	}
	if in.Kind != KindRedemption {
		t.Fatalf("kind = %q", in.Kind)
	}

	var payload RedemptionPayload
	if err := json.Unmarshal(in.Payload, &payload); err != nil {
		t.Fatalf("payload did not round trip: %v", err)
	}
	if payload.RewardTitle != "Granades!" || payload.RewardCost != 250 || payload.UserInput != "go on then" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestParseNotificationRejects(t *testing.T) {
	withID := func() http.Header {
		h := http.Header{}
		h.Set(HeaderMessageID, "msg-1")
		return h
	}

	t.Run("unsupported type is distinguishable", func(t *testing.T) {
		body := `{"subscription":{"type":"channel.follow","condition":{"broadcaster_user_id":"1"}},"event":{}}`
		_, err := ParseNotification(withID(), []byte(body))
		if !errors.Is(err, ErrUnsupportedType) {
			t.Fatalf("err = %v, want ErrUnsupportedType", err)
		}
	})

	t.Run("missing message id", func(t *testing.T) {
		body := `{"subscription":{"type":"channel.cheer","condition":{"broadcaster_user_id":"1"}},"event":{}}`
		if _, err := ParseNotification(http.Header{}, []byte(body)); err == nil {
			t.Fatal("accepted a notification with no message id")
		}
	})

	t.Run("missing broadcaster", func(t *testing.T) {
		body := `{"subscription":{"type":"channel.cheer","condition":{}},"event":{}}`
		if _, err := ParseNotification(withID(), []byte(body)); err == nil {
			t.Fatal("accepted a notification with no broadcaster_user_id")
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		if _, err := ParseNotification(withID(), []byte("{not json")); err == nil {
			t.Fatal("accepted malformed json")
		}
	})
}

func TestChallenge(t *testing.T) {
	got, err := Challenge([]byte(`{"challenge":"abc123","subscription":{"type":"channel.cheer"}}`))
	if err != nil {
		t.Fatalf("Challenge: %v", err)
	}
	if got != "abc123" {
		t.Fatalf("challenge = %q, want abc123", got)
	}

	if _, err := Challenge([]byte(`{"subscription":{}}`)); err == nil {
		t.Fatal("accepted a verification request with no challenge")
	}
}
