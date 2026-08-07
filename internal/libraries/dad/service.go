package dad

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

type NivekDadService interface {
	// PickRandom selects a random response from the channel's pool (globals plus
	// the channel's own rows), bumps its use_count, and returns the text. Returns
	// ("", nil) when the channel has no responses at all.
	PickRandom(channel string) (string, error)
	// ListForChannel returns the channel's pool (globals first, then its own),
	// for the web management page.
	ListForChannel(channel string) ([]DadResponse, error)
	// Add inserts a new channel-scoped response.
	Add(channel, response string) (DadResponse, error)
	// Remove deletes one of the channel's own responses. Globals and other
	// channels' rows are never affected.
	Remove(channel string, id int) error
}

type nivekDadServiceImpl struct {
	nivek    nivek.NivekService
	dadTable db.Collection
}

func NewService(service nivek.NivekService) NivekDadService {
	return &nivekDadServiceImpl{
		nivek:    service,
		dadTable: service.Postgres().GetDefaultConnection().Collection(TableDadResponse),
	}
}

func (s *nivekDadServiceImpl) PickRandom(channel string) (string, error) {
	// Pick + increment in one statement so usage is counted atomically and we
	// never fetch the whole pool to the bot. The subselect is the eligible pool.
	query := `
		UPDATE dad_response
		SET use_count = use_count + 1, updated_at = NOW()
		WHERE id = (
			SELECT id FROM dad_response
			WHERE is_global OR channelname = $1
			ORDER BY random()
			LIMIT 1
		)
		RETURNING response
	`

	res, err := s.dadTable.Session().SQL().QueryRow(query, channel)
	if err != nil {
		return "", fmt.Errorf("error picking dad response for channel %s: %w", channel, err)
	}

	var response string
	if errScan := res.Scan(&response); errScan != nil {
		if errors.Is(errScan, sql.ErrNoRows) {
			// No globals and no channel rows — nothing to say.
			return "", nil
		}
		return "", fmt.Errorf("error scanning dad response for channel %s: %w", channel, errScan)
	}

	return response, nil
}

func (s *nivekDadServiceImpl) ListForChannel(channel string) ([]DadResponse, error) {
	var responses []DadResponse

	err := s.dadTable.Find(db.Or(
		db.Cond{"is_global": true},
		db.Cond{"channelname": channel},
	)).OrderBy("is_global DESC", "id ASC").All(&responses)
	if err != nil {
		return nil, fmt.Errorf("error listing dad responses for channel %s: %w", channel, err)
	}

	return responses, nil
}

func (s *nivekDadServiceImpl) Add(channel, response string) (DadResponse, error) {
	rec := DadResponse{
		ChannelName: &channel,
		Response:    response,
		IsGlobal:    false,
		UseCount:    0,
	}

	result, err := s.dadTable.Insert(&rec)
	if err != nil {
		return DadResponse{}, fmt.Errorf("error adding dad response for channel %s: %w", channel, err)
	}

	if id, ok := result.ID().(int64); ok {
		rec.Id = int(id)
	}

	return rec, nil
}

func (s *nivekDadServiceImpl) Remove(channel string, id int) error {
	// Scope the delete to the channel's own rows: is_global = false guarantees a
	// channel can't delete a shared default, and channelname guarantees it can't
	// delete another channel's response.
	err := s.dadTable.Find(db.Cond{
		"id":          id,
		"channelname": channel,
		"is_global":   false,
	}).Delete()
	if err != nil {
		return fmt.Errorf("error removing dad response %d for channel %s: %w", id, channel, err)
	}

	return nil
}
