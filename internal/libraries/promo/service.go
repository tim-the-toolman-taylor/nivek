package promo

import (
	"errors"
	"fmt"
	"time"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

type NivekPromoService interface {
	// ListForChannel returns every promo owned by the channel (enabled or not),
	// for the dashboard management view.
	ListForChannel(channel string) ([]Promo, error)
	// ListActive returns all enabled promos across every channel — the set the
	// bot's scheduler polls.
	ListActive() ([]Promo, error)
	// Create inserts a new promo for the channel.
	Create(channel, message string, intervalSeconds int) (Promo, error)
	// Update rewrites the message/interval/enabled of one of the channel's own
	// promos. Scoped to the channel, so it can't touch another channel's rows.
	Update(channel string, id int, message string, intervalSeconds int, enabled bool) error
	// Remove deletes one of the channel's own promos.
	Remove(channel string, id int) error
	// UpdateLast rewrites the message + interval of the channel's most recently
	// touched promo (its enabled state is left as-is). Returns false when the
	// channel has no promos. Backs the `!newpromo edit-last` chat shortcut.
	UpdateLast(channel, message string, intervalSeconds int) (bool, error)
	// RemoveLast deletes the channel's most recently touched promo. Returns false
	// when the channel has no promos. Backs `!newpromo delete-last`.
	RemoveLast(channel string) (bool, error)
}

type nivekPromoServiceImpl struct {
	nivek      nivek.NivekService
	promoTable db.Collection
}

func NewService(service nivek.NivekService) NivekPromoService {
	return &nivekPromoServiceImpl{
		nivek:      service,
		promoTable: service.Postgres().GetDefaultConnection().Collection(TablePromo),
	}
}

// clampInterval keeps a caller-supplied interval inside the bounds the DB CHECK
// also enforces, so an out-of-range value becomes the nearest legal one instead
// of a failed insert.
func clampInterval(seconds int) int {
	if seconds < MinIntervalSeconds {
		return MinIntervalSeconds
	}
	if seconds > MaxIntervalSeconds {
		return MaxIntervalSeconds
	}
	return seconds
}

func (s *nivekPromoServiceImpl) ListForChannel(channel string) ([]Promo, error) {
	var promos []Promo
	if err := s.promoTable.Find(db.Cond{"channelname": channel}).OrderBy("id ASC").All(&promos); err != nil {
		return nil, fmt.Errorf("error listing promos for channel %s: %w", channel, err)
	}
	return promos, nil
}

func (s *nivekPromoServiceImpl) ListActive() ([]Promo, error) {
	var promos []Promo
	if err := s.promoTable.Find(db.Cond{"enabled": true}).All(&promos); err != nil {
		return nil, fmt.Errorf("error listing active promos: %w", err)
	}
	return promos, nil
}

func (s *nivekPromoServiceImpl) Create(channel, message string, intervalSeconds int) (Promo, error) {
	rec := Promo{
		Channelname:     channel,
		Message:         message,
		IntervalSeconds: clampInterval(intervalSeconds),
		Enabled:         true,
	}

	result, err := s.promoTable.Insert(&rec)
	if err != nil {
		return Promo{}, fmt.Errorf("error creating promo for channel %s: %w", channel, err)
	}

	if id, ok := result.ID().(int64); ok {
		rec.Id = int(id)
	}

	return rec, nil
}

func (s *nivekPromoServiceImpl) Update(channel string, id int, message string, intervalSeconds int, enabled bool) error {
	// Fetch scoped to (id, channelname): a promo owned by another channel simply
	// isn't found, so the update can never cross channel boundaries.
	var rec Promo
	if err := s.promoTable.Find(db.Cond{"id": id, "channelname": channel}).One(&rec); err != nil {
		return fmt.Errorf("promo %d not found for channel %s: %w", id, channel, err)
	}

	rec.Message = message
	rec.IntervalSeconds = clampInterval(intervalSeconds)
	rec.Enabled = enabled
	rec.UpdatedAt = time.Now()

	if err := s.promoTable.UpdateReturning(&rec); err != nil {
		return fmt.Errorf("error updating promo %d for channel %s: %w", id, channel, err)
	}
	return nil
}

func (s *nivekPromoServiceImpl) Remove(channel string, id int) error {
	if err := s.promoTable.Find(db.Cond{"id": id, "channelname": channel}).Delete(); err != nil {
		return fmt.Errorf("error removing promo %d for channel %s: %w", id, channel, err)
	}
	return nil
}

// mostRecent returns the channel's most recently touched promo (latest
// updated_at, ties broken by highest id for determinism). found is false when
// the channel has no promos at all.
func (s *nivekPromoServiceImpl) mostRecent(channel string) (rec Promo, found bool, err error) {
	err = s.promoTable.Find(db.Cond{"channelname": channel}).
		OrderBy("-updated_at", "-id").One(&rec)
	if err != nil {
		if errors.Is(err, db.ErrNoMoreRows) {
			return Promo{}, false, nil
		}
		return Promo{}, false, fmt.Errorf("error finding latest promo for channel %s: %w", channel, err)
	}
	return rec, true, nil
}

func (s *nivekPromoServiceImpl) UpdateLast(channel, message string, intervalSeconds int) (bool, error) {
	rec, found, err := s.mostRecent(channel)
	if err != nil || !found {
		return false, err
	}

	rec.Message = message
	rec.IntervalSeconds = clampInterval(intervalSeconds)
	rec.UpdatedAt = time.Now()

	if err := s.promoTable.UpdateReturning(&rec); err != nil {
		return false, fmt.Errorf("error updating latest promo for channel %s: %w", channel, err)
	}
	return true, nil
}

func (s *nivekPromoServiceImpl) RemoveLast(channel string) (bool, error) {
	rec, found, err := s.mostRecent(channel)
	if err != nil || !found {
		return false, err
	}

	if err := s.promoTable.Find(db.Cond{"id": rec.Id}).Delete(); err != nil {
		return false, fmt.Errorf("error removing latest promo for channel %s: %w", channel, err)
	}
	return true, nil
}
