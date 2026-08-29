package lurk

import (
	"errors"
	"fmt"
	"time"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

type NivekLurkService interface {
	OnMessage(channel, chatter string) int
}

type nivekLurkServiceImpl struct {
	nivek     nivek.NivekService
	lurkTable db.Collection
}

func NewService(service nivek.NivekService) NivekLurkService {
	return &nivekLurkServiceImpl{
		nivek:     service,
		lurkTable: service.Postgres().GetDefaultConnection().Collection(TableLurk),
	}
}

func (s *nivekLurkServiceImpl) OnMessage(channel, chatter string) int {
	lurker, err := s.getLurkerForMessage(channel, chatter)
	if err != nil {
		s.nivek.Logger().Errorf("[Lurk] failed to get or create lurker record! [%s - %s] %v", channel, chatter, err)
		return 0
	}

	lurker, err = s.incrementLurkCount(lurker)
	if err != nil {
		s.nivek.Logger().Errorf("[Lurk] failed to increment lurk count [%s - %s]: %v", channel, chatter, err)
		return 0
	}

	return lurker.LurkCount
}

func (s *nivekLurkServiceImpl) incrementLurkCount(lurker *Lurker) (*Lurker, error) {
	lurker.LurkCount = lurker.LurkCount + 1
	lurker.UpdatedAt = time.Now()

	err := s.lurkTable.UpdateReturning(lurker)
	if err != nil {
		return nil, fmt.Errorf("failed to update lurk record [%s - %s]: %w", lurker.ChannelName, lurker.ChatterName, err)
	}

	return lurker, nil
}

func (s *nivekLurkServiceImpl) getLurkerForMessage(channel, chatter string) (*Lurker, error) {
	var lurker Lurker

	err := s.lurkTable.Find(db.Cond{
		"channelname": channel,
		"chattername": chatter,
	}).One(&lurker)

	if err != nil {
		if !errors.Is(err, db.ErrNoMoreRows) {
			return nil, fmt.Errorf("failed to find or create lurker record: %w", err)
		}

		// Record doesn't exist - create it
		newLrkr := Lurker{
			ChannelName: channel,
			ChatterName: chatter,
			LurkCount:   0,
		}

		newLurker, errCreate := s.createLurker(&newLrkr)
		if errCreate != nil {
			return nil, errCreate
		}

		// Return the newly created record
		return newLurker, nil
	}

	return &lurker, err
}

func (s *nivekLurkServiceImpl) createLurker(newLurker *Lurker) (*Lurker, error) {
	// Insert the new record
	result, err := s.lurkTable.Insert(newLurker)
	if err != nil {
		return nil, fmt.Errorf("failed to create lurker record: %w", err)
	}

	// Get the auto-generated ID
	insertedID, ok := result.ID().(int64)
	if !ok {
		return nil, fmt.Errorf("failed to get inserted lurker ID")
	}

	return &Lurker{
		Id:          int(insertedID),
		ChannelName: newLurker.ChannelName,
		ChatterName: newLurker.ChatterName,
		LurkCount:   0,
	}, nil
}
