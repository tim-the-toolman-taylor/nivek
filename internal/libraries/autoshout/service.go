package autoshout

import (
	"fmt"
	"log"
	"time"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

type NivekAutoShoutService interface {
	GetAllAutoShoutChatters() ([]ShoutChatter, error)
	GetAutoShoutChatters(broadcasterId int) ([]ShoutChatter, error)
	GetAutoShoutChattersForBot(broadcasterId int) ([]string, error)
	GetAutoShoutChatter(broadcasterId int, chattername string) (*ShoutChatter, error)
	CreateAutoShoutChatter(broadcasterId int, chattername string) (int, error)
	UpdateAutoShoutChatter(chatter *ShoutChatter) error
	DeleteAutoShoutChatter(broadcasterId int, id int) error
}

type nivekAutoShoutServiceImpl struct {
	nivek      nivek.NivekService
	shoutTable db.Collection
}

func NewService(service nivek.NivekService) NivekAutoShoutService {
	return &nivekAutoShoutServiceImpl{
		nivek:      service,
		shoutTable: service.Postgres().GetDefaultConnection().Collection(TableShout),
	}
}

func formatAutoShoutChatters(shoutChatters []ShoutChatter) map[int]map[string]time.Time {
	result := make(map[int]map[string]time.Time)

	for _, chatter := range shoutChatters {
		if _, exists := result[chatter.TwitchID]; !exists {
			result[chatter.TwitchID] = make(map[string]time.Time)
		}

		result[chatter.TwitchID][chatter.ChatterName] = chatter.UpdatedAt
	}

	return result
}

func (s *nivekAutoShoutServiceImpl) incrementShoutCount(broadcasterId int, chatter string, lastShoutTime time.Time) {
	chatterRecord, err := s.GetAutoShoutChatter(broadcasterId, chatter)
	if err != nil {
		log.Printf("[AutoShout] failed to increment chatter score! %s", err.Error())
		return
	}

	chatterRecord.ShoutCount++
	chatterRecord.UpdatedAt = lastShoutTime

	err = s.UpdateAutoShoutChatter(chatterRecord)
	if err != nil {
		log.Printf("[AutoShout] failed to save incremented chatter score to the db! %s", err.Error())
		return
	}
}

func (s *nivekAutoShoutServiceImpl) GetAllAutoShoutChatters() ([]ShoutChatter, error) {
	var chatters []ShoutChatter

	if err := s.shoutTable.Find().All(&chatters); err != nil {
		return nil, fmt.Errorf("[AutoShout] error fetching all auto shout chatters %s", err.Error())
	}

	return chatters, nil
}

func (s *nivekAutoShoutServiceImpl) GetAutoShoutChatters(broadcasterId int) ([]ShoutChatter, error) {
	var chatters []ShoutChatter

	if err := s.shoutTable.Find(db.Cond{"twitch_id": broadcasterId}).All(&chatters); err != nil {
		return nil, fmt.Errorf("[AutoShout] error fetching auto shout chatters for channel %d - %s", broadcasterId, err.Error())
	}

	return chatters, nil
}

func (s *nivekAutoShoutServiceImpl) GetAutoShoutChattersForBot(broadcasterId int) ([]string, error) {
	// upper/db binds each row into a struct/map, so a bare []string fails with
	// "argument must be either a map or a struct". Select the single column into
	// a struct slice, then flatten to the []string the caller wants.
	var rows []struct {
		ChatterName string `db:"chattername"`
	}

	if err := s.shoutTable.Find(db.Cond{"twitch_id": broadcasterId}).
		Select("chattername").All(&rows); err != nil {
		return []string{}, fmt.Errorf("[AutoShout] error fetching chatters for channel %d - %s", broadcasterId, err.Error())
	}

	chatters := make([]string, len(rows))
	for i, r := range rows {
		chatters[i] = r.ChatterName
	}

	return chatters, nil
}

func (s *nivekAutoShoutServiceImpl) GetAutoShoutChatter(broadcasterId int, chattername string) (*ShoutChatter, error) {
	var chatter ShoutChatter

	if err := s.shoutTable.Find(db.Cond{
		"twitch_id":   broadcasterId,
		"chattername": chattername,
	}).One(&chatter); err != nil {
		return nil, fmt.Errorf("[AutoShout] error fetching auto shout chatter for channel %d chatter %s - %s",
			broadcasterId, chattername, err.Error(),
		)
	}

	return &chatter, nil
}

func (s *nivekAutoShoutServiceImpl) CreateAutoShoutChatter(broadcasterId int, chattername string) (int, error) {
	result, err := s.shoutTable.Insert(db.Cond{"twitch_id": broadcasterId, "chattername": chattername})
	if err != nil {
		return 0, fmt.Errorf(
			"[AutoShout] error creating auto shout chatter record for channel %d chatter %s - %s",
			broadcasterId,
			chattername,
			err.Error(),
		)
	}

	insertedID, ok := result.ID().(int64)
	if !ok {
		return 0, fmt.Errorf("[AutoShout] failed to get inserted ID")
	}

	return int(insertedID), nil
}

func (s *nivekAutoShoutServiceImpl) UpdateAutoShoutChatter(chatter *ShoutChatter) error {
	if err := s.shoutTable.UpdateReturning(chatter); err != nil {
		return fmt.Errorf("[AutoShout] error updating shout chatter record for channel %d chatter %s - %s", chatter.TwitchID, chatter.ChatterName, err.Error())
	}
	return nil
}

func (s *nivekAutoShoutServiceImpl) DeleteAutoShoutChatter(broadcasterId int, id int) error {
	if err := s.shoutTable.Find(db.Cond{"twitch_id": broadcasterId, "id": id}).Delete(); err != nil {
		return fmt.Errorf(
			"[AutoShout] error deleting auto shout chatter record for channel %d chatter id %d - %s",
			broadcasterId,
			id,
			err.Error(),
		)
	}

	return nil
}
