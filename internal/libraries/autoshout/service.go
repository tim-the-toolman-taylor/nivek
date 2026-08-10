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
	IncrementShoutCount(broadcasterId int, chattername string) error
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
	// Only chatters not yet shouted this stream: their last-shouted stream_key
	// differs from the broadcaster's current one. IS DISTINCT FROM handles the
	// NULLs (never-shouted rows, or a broadcaster with no current stream_key).
	const query = `
		SELECT a.chattername
		FROM nivek.auto_shout a
		JOIN nivek.users u ON u.twitch_id = a.twitch_id::text
		WHERE a.twitch_id = $1
		  AND a.stream_key IS DISTINCT FROM u.stream_key
	`
	rows, err := s.shoutTable.Session().SQL().Query(query, broadcasterId)
	if err != nil {
		return []string{}, fmt.Errorf("[AutoShout] error fetching chatters for channel %d - %s", broadcasterId, err.Error())
	}
	defer rows.Close()

	chatters := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return []string{}, fmt.Errorf("[AutoShout] error scanning chatter for channel %d - %s", broadcasterId, err.Error())
		}
		chatters = append(chatters, name)
	}
	if err := rows.Err(); err != nil {
		return []string{}, fmt.Errorf("[AutoShout] error iterating chatters for channel %d - %s", broadcasterId, err.Error())
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

// IncrementShoutCount bumps shout_count for one (broadcaster, chatter) row in a
// single atomic UPDATE. A missing row is a no-op (the shoutout only fires for
// chatters already on the list), so we don't upsert.
func (s *nivekAutoShoutServiceImpl) IncrementShoutCount(broadcasterId int, chattername string) error {
	// Stamp the chatter's row with the broadcaster's CURRENT stream_key (from
	// users) as we bump the count, so the fetch can tell they've been shouted
	// this stream.
	const query = `
		UPDATE nivek.auto_shout a
		SET shout_count = a.shout_count + 1,
		    stream_key = u.stream_key,
		    updated_at = NOW()
		FROM nivek.users u
		WHERE a.twitch_id = $1
		  AND a.chattername = $2
		  AND u.twitch_id = $1::text
	`
	if _, err := s.shoutTable.Session().SQL().Exec(query, broadcasterId, chattername); err != nil {
		return fmt.Errorf("[AutoShout] error incrementing shout count for channel %d chatter %s - %s", broadcasterId, chattername, err.Error())
	}

	return nil
}
