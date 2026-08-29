-- Overlay relay: durable per-broadcaster event log plus the device tokens that
-- authorize an overlay client to stream it.
--
-- Safe to re-run. Nothing reads these tables until the new core-api is live, so
-- this can be applied before or after the deploy.

-- A registered overlay client (one per machine the streamer runs the overlay on).
-- The token itself is never stored: we keep sha256(token) and compare hashes, so
-- a database disclosure does not hand out working credentials.
CREATE TABLE IF NOT EXISTS nivek.overlay_device (
    id SERIAL PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES nivek.users(id) ON DELETE CASCADE,
    token_hash   CHAR(64) NOT NULL UNIQUE,
    label        VARCHAR(100) NOT NULL DEFAULT '',
    created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMP,
    revoked_at   TIMESTAMP
);

CREATE INDEX IF NOT EXISTS overlay_device_user_idx ON nivek.overlay_device (user_id);

-- The event log. This is the source of truth; the websocket is only a delivery
-- optimisation. An overlay that was closed when a cheer landed replays it from
-- its cursor on the next connect.
--
-- seq is a per-user cursor assigned inside the insert rather than a BIGSERIAL.
-- Sequence values are allocated before commit, so under concurrent inserts a
-- lower id can become visible AFTER a higher one -- a cursor reader can step
-- past an event that had not committed yet and never see it again. Assigning
-- seq as MAX(seq)+1 under the UNIQUE (user_id, seq) constraint turns that race
-- into a loud unique violation the writer retries, instead of a silently
-- dropped event.
CREATE TABLE IF NOT EXISTS nivek.overlay_event (
    id BIGSERIAL PRIMARY KEY,
    user_id           INTEGER NOT NULL REFERENCES nivek.users(id) ON DELETE CASCADE,
    seq               BIGINT NOT NULL,
    twitch_message_id VARCHAR(64) NOT NULL,
    kind              VARCHAR(32) NOT NULL,
    payload           JSONB NOT NULL,
    created_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT overlay_event_message_uniq UNIQUE (user_id, twitch_message_id),
    CONSTRAINT overlay_event_seq_uniq     UNIQUE (user_id, seq)
);

CREATE INDEX IF NOT EXISTS overlay_event_cursor_idx ON nivek.overlay_event (user_id, seq);
CREATE INDEX IF NOT EXISTS overlay_event_created_idx ON nivek.overlay_event (created_at);
