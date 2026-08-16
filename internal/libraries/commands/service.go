package commands

import (
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

type CommandsService interface {
	GetCommands() ([]Commands, error)
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

func (s *commandsImpl) GetCommands() ([]Commands, error) {
	var commands []Commands

	if err := s.commandsTable.Find().All(&commands); err != nil {
		return nil, err
	}

	return commands, nil
}
