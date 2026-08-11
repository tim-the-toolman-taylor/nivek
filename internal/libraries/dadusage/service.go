package dadusage

import (
	"fmt"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

type NivekDadUsageService interface {
	// GetCurrentStreamUsage returns chatter -> rolls-served for the broadcaster's
	// CURRENT stream only. Rows stamped with a prior stream_key are omitted, so a
	// new stream starts everyone fresh even though the old rows linger. Used by the
	// bot to rehydrate its in-memory counters after a restart.
	GetCurrentStreamUsage(broadcasterId int) (map[string]int, error)
	// IncrementRoll records one !dad roll for a chatter, stamping the broadcaster's
	// current stream_key. If the stored row belongs to a previous stream (its
	// stream_key differs) the count resets to 1; otherwise it increments. A missing
	// users row is a no-op.
	IncrementRoll(broadcasterId int, chattername string) error
}

type nivekDadUsageServiceImpl struct {
	nivek      nivek.NivekService
	usageTable db.Collection
}

func NewService(service nivek.NivekService) NivekDadUsageService {
	return &nivekDadUsageServiceImpl{
		nivek:      service,
		usageTable: service.Postgres().GetDefaultConnection().Collection(TableDadUsage),
	}
}

func (s *nivekDadUsageServiceImpl) GetCurrentStreamUsage(broadcasterId int) (map[string]int, error) {
	// Only rows whose stamp equals the broadcaster's live stream_key. IS NOT
	// DISTINCT FROM handles the NULLs (a broadcaster with no current key matches
	// only NULL-key rows, of which there are none — so the result is empty, i.e.
	// no active stream => no counts).
	const query = `
		SELECT d.chattername, d.roll_count
		FROM nivek.dad_usage d
		JOIN nivek.users u ON u.twitch_id = d.twitch_id::text
		WHERE d.twitch_id = $1
		  AND d.stream_key IS NOT DISTINCT FROM u.stream_key
	`
	rows, err := s.usageTable.Session().SQL().Query(query, broadcasterId)
	if err != nil {
		return nil, fmt.Errorf("[DadUsage] error fetching usage for channel %d - %s", broadcasterId, err.Error())
	}
	defer rows.Close()

	usage := make(map[string]int)
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, fmt.Errorf("[DadUsage] error scanning usage for channel %d - %s", broadcasterId, err.Error())
		}
		usage[name] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("[DadUsage] error iterating usage for channel %d - %s", broadcasterId, err.Error())
	}

	return usage, nil
}

func (s *nivekDadUsageServiceImpl) IncrementRoll(broadcasterId int, chattername string) error {
	// Upsert keyed on (twitch_id, chattername). The proposed row carries the
	// broadcaster's live stream_key (from users); on conflict we reset to 1 when
	// that key differs from the stored one (a new stream) and otherwise bump. This
	// keeps the persisted per-stream count in lockstep with the bot's in-memory
	// counter, which is likewise reset each go-live.
	const query = `
		INSERT INTO nivek.dad_usage (twitch_id, chattername, roll_count, stream_key)
		SELECT $1::integer, $2, 1, u.stream_key
		FROM nivek.users u
		WHERE u.twitch_id = $1::integer::text
		ON CONFLICT (twitch_id, chattername) DO UPDATE
		SET roll_count = CASE
		        WHEN dad_usage.stream_key IS DISTINCT FROM EXCLUDED.stream_key THEN 1
		        ELSE dad_usage.roll_count + 1
		    END,
		    stream_key = EXCLUDED.stream_key,
		    updated_at = NOW()
	`
	if _, err := s.usageTable.Session().SQL().Exec(query, broadcasterId, chattername); err != nil {
		return fmt.Errorf("[DadUsage] error incrementing roll for channel %d chatter %s - %s", broadcasterId, chattername, err.Error())
	}

	return nil
}
