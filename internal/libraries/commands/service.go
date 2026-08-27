package commands

import (
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

type CommandsService interface {
	GetGlobalEnabledCommands() ([]Commands, error)
	GetChannelEnabledCommands(channelTwitchID string) ([]Commands, error)
}

type commandsImpl struct {
	nivek         nivek.NivekService
	commandsTable db.Collection
}

func NewService(service nivek.NivekService) CommandsService {
	return &commandsImpl{
		nivek:         service,
		commandsTable: service.Postgres().GetDefaultConnection().Collection(TableCommands),
	}
}

func (s *commandsImpl) GetGlobalEnabledCommands() ([]Commands, error) {
	var commands []Commands

	if err := s.commandsTable.Find(db.Cond{
		"scope":   "global",
		"enabled": true,
	}).All(&commands); err != nil {
		return nil, err
	}

	return commands, nil
}

// GetChannelEnabledCommands returns the enabled channel-scoped (per-channel
// custom) commands for one broadcaster, keyed by their Twitch id. These are the
// rows the bot loads when a channel goes live so its custom triggers respond.
// Global commands are excluded — those load once at boot via
// GetGlobalEnabledCommands.
func (s *commandsImpl) GetChannelEnabledCommands(channelTwitchID string) ([]Commands, error) {
	var commands []Commands

	if err := s.commandsTable.Find(db.Cond{
		"scope":             "channel",
		"channel_twitch_id": channelTwitchID,
		"enabled":           true,
	}).All(&commands); err != nil {
		return nil, err
	}

	return commands, nil
}
