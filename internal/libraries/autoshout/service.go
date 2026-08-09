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
	GetAutoShoutChatters(channelname string) ([]ShoutChatter, error)
	GetAutoShoutChattersForBot(broadcasterId string) ([]string, error)
	GetAutoShoutChatter(channelname, chattername string) (*ShoutChatter, error)
	CreateAutoShoutChatter(channelname, chattername string) (int, error)
	UpdateAutoShoutChatter(chatter *ShoutChatter) error
	DeleteAutoShoutChatter(channelname string, chattername int) error
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

func formatAutoShoutChatters(shoutChatters []ShoutChatter) map[string]map[string]time.Time {
	result := make(map[string]map[string]time.Time)

	for _, chatter := range shoutChatters {
		if _, exists := result[chatter.ChannelName]; !exists {
			result[chatter.ChannelName] = make(map[string]time.Time)
		}

		result[chatter.ChannelName][chatter.ChatterName] = chatter.UpdatedAt
	}

	return result
}

func (s *nivekAutoShoutServiceImpl) incrementShoutCount(channel, chatter string, lastShoutTime time.Time) {
	chatterRecord, err := s.GetAutoShoutChatter(channel, chatter)
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

func (s *nivekAutoShoutServiceImpl) GetAutoShoutChatters(channelname string) ([]ShoutChatter, error) {
	var chatters []ShoutChatter

	if err := s.shoutTable.Find(db.Cond{"channelname": channelname}).All(&chatters); err != nil {
		return nil, fmt.Errorf("[AutoShout] error fetching auto shout chatters for channel %s - %s", channelname, err.Error())
	}

	return chatters, nil
}

func (s *nivekAutoShoutServiceImpl) GetAutoShoutChattersForBot(broadcasterId string) ([]string, error) {
	var chatters []string

	if err := s.shoutTable.Find(db.Cond{"twitch_id": broadcasterId}).All(&chatters); err != nil {
		return []string{}, fmt.Errorf("[AutoShout] error fetching chatters for channel %s - %s", broadcasterId, err.Error())
	}

	return chatters, nil
}

func (s *nivekAutoShoutServiceImpl) GetAutoShoutChatter(channelname, chattername string) (*ShoutChatter, error) {
	var chatter ShoutChatter

	if err := s.shoutTable.Find(db.Cond{
		"channelname": channelname,
		"chattername": chattername,
	}).One(&chatter); err != nil {
		return nil, fmt.Errorf("[AutoShout] error fetching auto shout chatter for channel %s chatter %s - %s",
			channelname, chattername, err.Error(),
		)
	}

	return &chatter, nil
}

func (s *nivekAutoShoutServiceImpl) CreateAutoShoutChatter(channelname, chattername string) (int, error) {
	result, err := s.shoutTable.Insert(db.Cond{"channelname": channelname, "chattername": chattername})
	if err != nil {
		return 0, fmt.Errorf(
			"[AutoShout] error creating auto shout chatter record for channel %s chatter %s - %s",
			channelname,
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
		return fmt.Errorf("[AutoShout] error updating shout chatter record for channel %s chatter %s - %s", chatter.ChannelName, chatter.ChatterName, err.Error())
	}
	return nil
}

func (s *nivekAutoShoutServiceImpl) DeleteAutoShoutChatter(channelname string, id int) error {
	if err := s.shoutTable.Find(db.Cond{"channelname": channelname, "id": id}).Delete(); err != nil {
		return fmt.Errorf(
			"[AutoShout] error deleting auto shout chatter record for channel %s chatter id %d - %s",
			channelname,
			id,
			err.Error(),
		)
	}

	return nil
}
