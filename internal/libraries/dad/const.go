package dad

import "time"

const TableDadResponse = "dad_response"

// DadResponse is one possible !dad reply. IsGlobal rows (ChannelName nil) are the
// shared defaults available in every channel; non-global rows belong to the
// channel named by ChannelName (a lowercased Twitch login).
type DadResponse struct {
	Id          int       `db:"id,omitempty" json:"id"`
	ChannelName *string   `db:"channelname" json:"channelname"`
	Response    string    `db:"response" json:"response"`
	IsGlobal    bool      `db:"is_global" json:"is_global"`
	UseCount    int       `db:"use_count" json:"use_count"`
	UpdatedAt   time.Time `db:"updated_at,omitempty" json:"updated_at"`
	CreatedAt   time.Time `db:"created_at,omitempty" json:"created_at"`
}
