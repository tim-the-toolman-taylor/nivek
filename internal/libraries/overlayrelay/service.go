package overlayrelay

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
	"github.com/upper/db/v4"
)

// ErrUnknownBroadcaster means a verified notification arrived for a Twitch
// channel with no local user row. Subscriptions outlive the account that
// created them, so this is expected during cleanup, not a fault.
var ErrUnknownBroadcaster = errors.New("no local user for broadcaster")

// ErrDeviceNotFound covers both "no such token" and "token was revoked". The
// two are deliberately indistinguishable to a caller holding a bad token.
var ErrDeviceNotFound = errors.New("device token not recognised")

// seqRetries bounds the retry loop when two notifications for the same
// broadcaster race for the next cursor position.
const seqRetries = 5

// MaxReplay caps how much backlog one reconnect will replay, so an overlay that
// has been offline for a month cannot pin the process building one response.
const MaxReplay = 500

type Service interface {
	Ingest(in Incoming) (Event, bool, error)
	EventsAfter(userID int, since int64, limit int) ([]Event, error)

	CreateDevice(userID int, label string) (string, Device, error)
	AuthenticateDevice(token string) (*Device, error)
	ListDevices(userID int) ([]Device, error)
	RevokeDevice(userID int, deviceID int) error
}

type service struct {
	nivek nivek.NivekService
	users user.NivekUserService
}

func NewService(svc nivek.NivekService) Service {
	return &service{nivek: svc, users: user.NewService(svc)}
}

func (s *service) session() db.Session {
	return s.nivek.Postgres().GetDefaultConnection()
}

// Ingest resolves the broadcaster to a local user and appends the event.
//
// The bool reports whether this was new. Twitch retries deliveries whenever an
// acknowledgement is slow or lost, so the same notification arriving twice is
// routine -- and for a paid interaction, acting on it twice is the one failure
// that cannot be undone. Deduplication therefore lives here, at the durable
// boundary, rather than anywhere downstream.
func (s *service) Ingest(in Incoming) (Event, bool, error) {
	broadcaster, err := s.users.GetUserByBroadcasterId(in.BroadcasterUserID)
	if err != nil {
		// A genuine no-such-row is the expected "subscription outlived the
		// account" case: report it as ErrUnknownBroadcaster so the handler acks
		// (204) and Twitch stops retrying. Any OTHER error is a transient DB
		// failure -- surface it unchanged so the handler returns 5xx and Twitch
		// retries, rather than acking and losing a paid event forever.
		if errors.Is(err, db.ErrNoMoreRows) {
			return Event{}, false, fmt.Errorf("%w: %s", ErrUnknownBroadcaster, in.BroadcasterUserID)
		}
		return Event{}, false, fmt.Errorf("resolve broadcaster %s: %w", in.BroadcasterUserID, err)
	}
	if broadcaster == nil {
		// Defensive: .One() never returns (nil, nil), but a nil user with no
		// error still means we have nowhere to attribute the event.
		return Event{}, false, fmt.Errorf("%w: %s", ErrUnknownBroadcaster, in.BroadcasterUserID)
	}

	for attempt := 0; attempt < seqRetries; attempt++ {
		event, inserted, err := s.appendEvent(broadcaster.Id, in)
		if err == nil {
			return event, inserted, nil
		}
		if !isSeqConflict(err) {
			return Event{}, false, err
		}
		// Another notification for this broadcaster took the cursor position
		// between our MAX(seq) read and the insert. Recompute and retry.
	}

	return Event{}, false, fmt.Errorf("append event: exhausted %d attempts competing for a cursor position", seqRetries)
}

// appendEvent assigns the next per-user cursor position and inserts.
//
// The seq subquery and the insert are one statement so they share a snapshot;
// the UNIQUE (user_id, seq) constraint converts a lost race into an error the
// caller retries, instead of two events quietly sharing a position and one of
// them becoming unreachable to a cursor reader.
func (s *service) appendEvent(userID int, in Incoming) (Event, bool, error) {
	const query = `
		INSERT INTO nivek.overlay_event (user_id, seq, twitch_message_id, kind, payload)
		SELECT $1, COALESCE(MAX(seq), 0) + 1, $2, $3, $4::jsonb
		  FROM nivek.overlay_event WHERE user_id = $1
		ON CONFLICT ON CONSTRAINT overlay_event_message_uniq DO NOTHING
		RETURNING seq, created_at`

	row, err := s.session().SQL().QueryRow(query, userID, in.TwitchMessageID, string(in.Kind), string(in.Payload))
	if err != nil {
		return Event{}, false, fmt.Errorf("append event: %w", err)
	}

	var (
		seq       int64
		createdAt time.Time
	)
	if err := row.Scan(&seq, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// ON CONFLICT DO NOTHING fired: we already have this message.
			return Event{}, false, nil
		}
		return Event{}, false, fmt.Errorf("append event: %w", err)
	}

	return Event{
		UserId:          userID,
		Seq:             seq,
		TwitchMessageId: in.TwitchMessageID,
		Kind:            in.Kind,
		Payload:         in.Payload,
		CreatedAt:       createdAt,
	}, true, nil
}

// isSeqConflict distinguishes losing the race for a cursor position (retryable)
// from any other database failure (not).
func isSeqConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "overlay_event_seq_uniq")
}

// EventsAfter returns the backlog an overlay missed, oldest first.
func (s *service) EventsAfter(userID int, since int64, limit int) ([]Event, error) {
	if limit <= 0 || limit > MaxReplay {
		limit = MaxReplay
	}

	const query = `
		SELECT seq, twitch_message_id, kind, payload, created_at
		  FROM nivek.overlay_event
		 WHERE user_id = $1 AND seq > $2
		 ORDER BY seq ASC
		 LIMIT $3`

	rows, err := s.session().SQL().Query(query, userID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("read events after %d: %w", since, err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var (
			event   Event
			payload []byte
		)
		if err := rows.Scan(&event.Seq, &event.TwitchMessageId, &event.Kind, &payload, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		event.UserId = userID
		event.Payload = json.RawMessage(payload)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read events after %d: %w", since, err)
	}
	return events, nil
}

// CreateDevice mints a token for userID. The plaintext is returned exactly
// once; only its hash is persisted.
func (s *service) CreateDevice(userID int, label string) (string, Device, error) {
	token, hash, err := MintToken()
	if err != nil {
		return "", Device{}, err
	}

	device := Device{
		UserId:    userID,
		TokenHash: hash,
		Label:     strings.TrimSpace(label),
		CreatedAt: time.Now().UTC(),
	}
	result, err := deviceTable(s.nivek).Insert(device)
	if err != nil {
		return "", Device{}, fmt.Errorf("create overlay device: %w", err)
	}
	// Populate the generated primary key so the mint response carries the real
	// device id (callers revoke/display by it); a bare Insert leaves it zero.
	if id, ok := result.ID().(int64); ok {
		device.Id = int(id)
	}
	return token, device, nil
}

// AuthenticateDevice resolves a presented token and records the sighting.
func (s *service) AuthenticateDevice(token string) (*Device, error) {
	if !LooksLikeToken(token) {
		return nil, ErrDeviceNotFound
	}

	var device Device
	err := deviceTable(s.nivek).
		Find(db.Cond{"token_hash": HashToken(token), "revoked_at": nil}).
		One(&device)
	if err != nil {
		if errors.Is(err, db.ErrNoMoreRows) {
			return nil, ErrDeviceNotFound
		}
		return nil, fmt.Errorf("authenticate overlay device: %w", err)
	}

	// Best effort: a failed sighting update must not deny a valid connection.
	now := time.Now().UTC()
	if err := deviceTable(s.nivek).Find(db.Cond{"id": device.Id}).Update(map[string]any{"last_seen_at": now}); err != nil {
		s.nivek.Logger().Warnf("overlay device %d: could not record last_seen_at: %s", device.Id, err.Error())
	} else {
		device.LastSeenAt = &now
	}

	return &device, nil
}

func (s *service) ListDevices(userID int) ([]Device, error) {
	var devices []Device
	err := deviceTable(s.nivek).
		Find(db.Cond{"user_id": userID, "revoked_at": nil}).
		OrderBy("created_at").
		All(&devices)
	if err != nil {
		return nil, fmt.Errorf("list overlay devices: %w", err)
	}
	return devices, nil
}

// RevokeDevice is scoped by user so one streamer cannot revoke another's token
// by guessing an id.
func (s *service) RevokeDevice(userID int, deviceID int) error {
	err := deviceTable(s.nivek).
		Find(db.Cond{"id": deviceID, "user_id": userID, "revoked_at": nil}).
		Update(map[string]any{"revoked_at": time.Now().UTC()})
	if err != nil {
		if errors.Is(err, db.ErrNoMoreRows) {
			return ErrDeviceNotFound
		}
		return fmt.Errorf("revoke overlay device %d: %w", deviceID, err)
	}
	return nil
}
