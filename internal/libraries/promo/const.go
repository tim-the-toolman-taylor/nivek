package promo

import "time"

const TablePromo = "promo"

// Interval bounds shared by the service (clamp on write) and the bot's
// scheduler (floor at read). Kept here so the DB CHECK, the API validation, and
// the chat command all agree on the same limits.
const (
	MinIntervalSeconds = 60    // 1 minute — anything tighter is spam
	MaxIntervalSeconds = 86400 // 24 hours
)

// Promo is one recurring message. channelname is the lowercased Twitch login of
// the channel the message is posted in (the same value the bot sees as
// message.Channel), so a promo created from chat and one created from the
// dashboard land on the same row set.
type Promo struct {
	Id              int       `db:"id,omitempty" json:"id"`
	Channelname     string    `db:"channelname" json:"channelname"`
	BroadcasterId   string    `db:"broadcaster_id" json:"broadcaster_id"`
	Message         string    `db:"message" json:"message"`
	IntervalSeconds int       `db:"interval_seconds" json:"interval_seconds"`
	Enabled         bool      `db:"enabled" json:"enabled"`
	UpdatedAt       time.Time `db:"updated_at,omitempty" json:"updated_at"`
	CreatedAt       time.Time `db:"created_at,omitempty" json:"created_at"`
}
