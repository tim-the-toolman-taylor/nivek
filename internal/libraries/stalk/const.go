package stalk

import "time"

const TableStalk = "stalk"

// Stalk is the per-channel target for !stalk. channelname is the lowercased
// Twitch login of the channel (message.Channel), target_login is the lowercased
// login/display-name the channel is stalking.
type Stalk struct {
	Id          int       `db:"id,omitempty" json:"id"`
	Channelname string    `db:"channelname" json:"channelname"`
	TargetLogin string    `db:"target_login" json:"target_login"`
	SetBy       string    `db:"set_by" json:"set_by"`
	CreatedAt   time.Time `db:"created_at,omitempty" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at,omitempty" json:"updated_at"`
}
