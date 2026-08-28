package stalk

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

type NivekStalkService interface {
	// Get returns the channel's stalk row. found is false when the channel has
	// no row (not configured yet).
	Get(channel string) (Stalk, bool, error)
	// Set upserts the channel's stalk target. channel and target are stored
	// lowercased. setBy is the login of the mod/broadcaster who set it.
	// Changing the target clears last_message so we don't quote the previous
	// chatter.
	Set(channel, target, setBy string) error
	// SetLastMessage writes the target's latest chat line. No-op (found=false)
	// when the channel has no stalk row, or when target no longer matches
	// (a dashboard/chat retarget landed first).
	SetLastMessage(channel, target, message string) (bool, error)
	// Clear deletes the channel's stalk row. found is false when there was
	// nothing to delete.
	Clear(channel string) (found bool, err error)
}

type nivekStalkServiceImpl struct {
	nivek      nivek.NivekService
	stalkTable db.Collection
}

func NewService(service nivek.NivekService) NivekStalkService {
	return &nivekStalkServiceImpl{
		nivek:      service,
		stalkTable: service.Postgres().GetDefaultConnection().Collection(TableStalk),
	}
}

func normalizeChannel(channel string) string {
	return strings.ToLower(strings.TrimSpace(channel))
}

func (s *nivekStalkServiceImpl) Get(channel string) (Stalk, bool, error) {
	channel = normalizeChannel(channel)
	if channel == "" {
		return Stalk{}, false, fmt.Errorf("channel required")
	}

	var rec Stalk
	err := s.stalkTable.Find(db.Cond{"channelname": channel}).One(&rec)
	if err != nil {
		if errors.Is(err, db.ErrNoMoreRows) {
			return Stalk{}, false, nil
		}
		return Stalk{}, false, fmt.Errorf("error fetching stalk target for %s: %w", channel, err)
	}
	return rec, true, nil
}

func (s *nivekStalkServiceImpl) Set(channel, target, setBy string) error {
	channel = normalizeChannel(channel)
	target = strings.ToLower(strings.TrimSpace(target))
	setBy = strings.ToLower(strings.TrimSpace(setBy))
	if channel == "" || target == "" {
		return fmt.Errorf("channel and target required")
	}

	var rec Stalk
	err := s.stalkTable.Find(db.Cond{"channelname": channel}).One(&rec)
	if err != nil {
		if !errors.Is(err, db.ErrNoMoreRows) {
			return fmt.Errorf("error looking up stalk target for %s: %w", channel, err)
		}
		rec = Stalk{
			Channelname: channel,
			TargetLogin: target,
			SetBy:       setBy,
		}
		if _, errInsert := s.stalkTable.Insert(&rec); errInsert != nil {
			return fmt.Errorf("error creating stalk target for %s: %w", channel, errInsert)
		}
		return nil
	}

	if rec.TargetLogin != target {
		rec.LastMessage = ""
	}
	rec.TargetLogin = target
	rec.SetBy = setBy
	rec.UpdatedAt = time.Now()
	if err := s.stalkTable.UpdateReturning(&rec); err != nil {
		return fmt.Errorf("error updating stalk target for %s: %w", channel, err)
	}
	return nil
}

func (s *nivekStalkServiceImpl) SetLastMessage(channel, target, message string) (bool, error) {
	channel = normalizeChannel(channel)
	target = strings.ToLower(strings.TrimSpace(target))
	message = strings.TrimSpace(message)
	if channel == "" {
		return false, fmt.Errorf("channel required")
	}

	var rec Stalk
	err := s.stalkTable.Find(db.Cond{"channelname": channel}).One(&rec)
	if err != nil {
		if errors.Is(err, db.ErrNoMoreRows) {
			return false, nil
		}
		return false, fmt.Errorf("error looking up stalk target for %s: %w", channel, err)
	}
	if target != "" && rec.TargetLogin != target {
		return false, nil
	}

	rec.LastMessage = message
	rec.UpdatedAt = time.Now()
	if err := s.stalkTable.UpdateReturning(&rec); err != nil {
		return false, fmt.Errorf("error updating last message for %s: %w", channel, err)
	}
	return true, nil
}

func (s *nivekStalkServiceImpl) Clear(channel string) (bool, error) {
	channel = normalizeChannel(channel)
	if channel == "" {
		return false, fmt.Errorf("channel required")
	}

	var rec Stalk
	err := s.stalkTable.Find(db.Cond{"channelname": channel}).One(&rec)
	if err != nil {
		if errors.Is(err, db.ErrNoMoreRows) {
			return false, nil
		}
		return false, fmt.Errorf("error looking up stalk target for %s: %w", channel, err)
	}

	if err := s.stalkTable.Find(db.Cond{"id": rec.Id}).Delete(); err != nil {
		return false, fmt.Errorf("error clearing stalk target for %s: %w", channel, err)
	}
	return true, nil
}
