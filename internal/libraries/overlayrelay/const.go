// Package overlayrelay delivers Twitch monetisation events (cheers, channel
// point redemptions) to a streamer's overlay application running on their own
// machine.
//
// The overlay cannot be reached from here: it sits behind residential NAT. So
// it dials out over wss:// and holds the connection, and we push down it. That
// also means the socket is unreliable by nature -- the overlay is closed
// between streams, restarts, loses wifi -- so every event is written to a
// durable per-broadcaster log first and pushed second. The overlay tracks a
// cursor and replays anything it missed on reconnect.
package overlayrelay

import (
	"encoding/json"
	"time"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

const (
	TableDevice = "overlay_device"
	TableEvent  = "overlay_event"
)

func deviceTable(svc nivek.NivekService) db.Collection {
	return svc.Postgres().GetDefaultConnection().Collection(TableDevice)
}

// Kind discriminates the event payloads an overlay can act on.
type Kind string

const (
	KindCheer      Kind = "cheer"
	KindRedemption Kind = "redemption"
	KindPowerUp    Kind = "power_up"
	KindExtension  Kind = "extension"
	// KindCommand is a chat command routed to the overlay for execution. Unlike
	// every other kind it is NOT replayed on reconnect -- see EventsAfter.
	KindCommand Kind = "command"
)

// Device is one registered overlay client. Token holds sha256(token) hex; the
// plaintext exists only in the response to the mint call.
type Device struct {
	Id     int `db:"id,omitempty" json:"id"`
	UserId int `db:"user_id" json:"user_id"`
	// StartSeq is the per-user event seq this device begins at. Replay is floored
	// to it so a freshly minted device (which connects with since=0) never
	// re-executes events stored before it existed. Set to the user's MAX(seq) at
	// mint time; existing/legacy devices carry 0 and replay from their own cursor.
	StartSeq   int64      `db:"start_seq" json:"-"`
	TokenHash  string     `db:"token_hash" json:"-"`
	Label      string     `db:"label" json:"label"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	LastSeenAt *time.Time `db:"last_seen_at" json:"last_seen_at,omitempty"`
	RevokedAt  *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
}

// Event is one stored notification.
type Event struct {
	Id              int64           `db:"id,omitempty" json:"-"`
	UserId          int             `db:"user_id" json:"-"`
	Seq             int64           `db:"seq" json:"seq"`
	TwitchMessageId string          `db:"twitch_message_id" json:"id"`
	Kind            Kind            `db:"kind" json:"kind"`
	Payload         json.RawMessage `db:"payload" json:"data"`
	CreatedAt       time.Time       `db:"created_at" json:"ts"`
}

// Incoming is a verified EventSub notification on its way into the log, before
// it has been matched to a local user or assigned a cursor position.
type Incoming struct {
	// TwitchMessageID is the Twitch-Eventsub-Message-Id header. Twitch retries
	// deliveries, so this is the idempotency key -- not the payload contents.
	TwitchMessageID   string
	BroadcasterUserID string
	Kind              Kind
	Payload           json.RawMessage
}

// CheerPayload is the subset of channel.cheer an overlay needs. Anonymous
// cheers arrive with no user identity at all, so those fields stay empty.
type CheerPayload struct {
	UserID      string `json:"user_id,omitempty"`
	UserLogin   string `json:"user_login,omitempty"`
	UserName    string `json:"user_name,omitempty"`
	IsAnonymous bool   `json:"is_anonymous"`
	Bits        int    `json:"bits"`
	Message     string `json:"message"`
}

// RedemptionPayload is the subset of
// channel.channel_points_custom_reward_redemption.add an overlay needs.
type RedemptionPayload struct {
	UserID      string `json:"user_id"`
	UserLogin   string `json:"user_login"`
	UserName    string `json:"user_name"`
	RewardID    string `json:"reward_id"`
	RewardTitle string `json:"reward_title"`
	RewardCost  int    `json:"reward_cost"`
	UserInput   string `json:"user_input"`
	Status      string `json:"status"`
}

// PowerUpPayload is the subset of channel.custom_power_up_redemption.add an
// overlay needs. Custom Power-ups are paid with Bits; PowerUpTitle is what the
// overlay dispatches on (like RewardTitle for channel-point redemptions).
type PowerUpPayload struct {
	UserID       string `json:"user_id"`
	UserLogin    string `json:"user_login"`
	UserName     string `json:"user_name"`
	Bits         int    `json:"bits"`
	PowerUpID    string `json:"power_up_id"`
	PowerUpTitle string `json:"power_up_title"`
}

// CommandPayload is a chat command the bot has routed here for the overlay to
// execute. Action is the stable name the overlay dispatches on (matching the
// handler_key in nivek.command minus the "overlay_" prefix); Args are the
// command's trailing words, already split.
type CommandPayload struct {
	Action    string   `json:"action"`
	Args      []string `json:"args,omitempty"`
	UserID    string   `json:"user_id,omitempty"`
	UserLogin string   `json:"user_login"`
	UserName  string   `json:"user_name"`
}

// ExtensionPayload is a Bits interaction redeemed through the Twitch Extension
// (a Bits-in-Extensions product purchase), not an EventSub webhook. It reaches
// the relay from the extension's backend endpoint after the signed Bits receipt
// is verified. ProductSKU is what the overlay dispatches on (like PowerUpTitle).
// UserLogin/UserName stay empty: a Bits receipt names the purchaser only by
// numeric id, and identity is not shared by default.
type ExtensionPayload struct {
	UserID      string `json:"user_id,omitempty"`
	UserLogin   string `json:"user_login,omitempty"`
	UserName    string `json:"user_name,omitempty"`
	Bits        int    `json:"bits"`
	ProductSKU  string `json:"product_sku"`
	ProductName string `json:"product_name,omitempty"`
}

// Wire protocol, overlay <-> relay.

const (
	// Client -> server.
	MsgHello = "hello"
	MsgAck   = "ack"
	MsgPing  = "ping"

	// Server -> client.
	MsgReady = "ready"
	MsgEvent = "event"
	MsgPong  = "pong"
)

// Hello is the first frame the overlay sends. Since is the last seq it durably
// processed; the relay replays everything after it before going live.
type Hello struct {
	Type  string `json:"type"`
	Token string `json:"token"`
	Since int64  `json:"since"`
}

// ClientFrame is any subsequent frame from the overlay.
type ClientFrame struct {
	Type string `json:"type"`
	Seq  int64  `json:"seq,omitempty"`
}

// ServerFrame is anything the relay sends down. Event is set only for
// MsgEvent frames -- it is a named field rather than an embedded one because
// encoding/json cannot marshal a nil embedded struct pointer.
type ServerFrame struct {
	Type  string `json:"type"`
	Event *Event `json:"event,omitempty"`
}
