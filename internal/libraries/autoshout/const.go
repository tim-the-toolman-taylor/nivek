package autoshout

import "time"

const TableShout = "auto_shout"

type ShoutChatter struct {
	Id          int       `db:"id" json:"id"`
	TwitchID    int       `db:"twitch_id" json:"twitch_id"`
	ChatterName string    `db:"chattername" json:"chattername"`
	ShoutCount  int       `db:"shout_count" json:"shout_count"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}
