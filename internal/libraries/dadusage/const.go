package dadusage

import "time"

const TableDadUsage = "dad_usage"

// DadUsage is one chatter's !dad roll tally for a broadcaster's current stream.
// roll_count is per-stream (not lifetime): stream_key holds the users.stream_key
// value in force when the count was last touched, so a new stream (fresh key)
// resets the chatter's allotment. This is the restart-survivable mirror of the
// in-memory dadStreamUsage counters the bot keeps.
type DadUsage struct {
	Id          int       `db:"id,omitempty" json:"id"`
	TwitchID    int       `db:"twitch_id" json:"twitch_id"`
	ChatterName string    `db:"chattername" json:"chattername"`
	RollCount   int       `db:"roll_count" json:"roll_count"`
	StreamKey   *string   `db:"stream_key" json:"stream_key"`
	CreatedAt   time.Time `db:"created_at,omitempty" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at,omitempty" json:"updated_at"`
}
