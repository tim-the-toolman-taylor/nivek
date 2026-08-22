package commands

import "time"

const TableCommands = "command"

type Commands struct {
	Id              int       `db:"id" json:"id"`
	Trigger         string    `db:"trigger" json:"trigger"`
	Kind            string    `db:"kind" json:"kind"`
	HandlerKey      *string   `db:"handler_key,omitempty" json:"handler_key"`
	ResponseTmpl    *string   `db:"response_tmpl,omitempty" json:"response_tmpl"`
	Description     string    `db:"description" json:"description"`
	Scope           string    `db:"scope" json:"scope"`
	ChannelTwitchId *string   `db:"channel_twitch_id,omitempty" json:"channel_twitch_id"`
	Enabled         bool      `db:"enabled" json:"enabled"`
	MinRole         string    `db:"min_role" json:"min_role"`
	CooldownSecs    int       `db:"cooldown_secs" json:"cooldown_secs"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}
