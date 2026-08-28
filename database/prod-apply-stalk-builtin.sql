-- Production apply for the !stalk global-builtin change.
-- Safe to re-run: CREATE IF NOT EXISTS, INSERT ON CONFLICT DO NOTHING, DELETE is
-- idempotent once the mystasuga custom row is gone.
--
-- Deploy order (important):
--   1. CREATE TABLE can run at any time — nothing reads nivek.stalk until the
--      new core-api is live.
--   2. Deploy / merge this branch FIRST. The current bot fails boot on an
--      unknown handler_key, so inserting the global !stalk row while the old
--      binary is still running will crash the bot on its next restart.
--   3. Then run the INSERT + DELETE below.
--
-- What this does:
--   1. Creates nivek.stalk (one target chatter per channel).
--   2. Seeds the global builtin !stalk row (handler_key = 'stalk').
--   3. Removes the mystasuga-only custom !stalk command so it can't linger next
--      to the global builtin (the builtin would win on a clash anyway).

CREATE TABLE IF NOT EXISTS nivek.stalk (
    id SERIAL PRIMARY KEY,
    channelname  VARCHAR(50) NOT NULL,
    target_login VARCHAR(50) NOT NULL,
    set_by       VARCHAR(50),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT stalk_channel_uk UNIQUE (channelname)
);

INSERT INTO nivek.command (trigger, kind, handler_key, min_role, description)
VALUES (
    '!stalk',
    'builtin',
    'stalk',
    'everyone',
    'Quotes the last chat message from the chatter this channel is stalking. Anyone can run it; mods/broadcaster pick the target with "!stalk set <username>" and clear it with "!stalk clear".'
)
ON CONFLICT (trigger) WHERE scope = 'global' DO NOTHING;

DELETE FROM nivek.command
 WHERE scope = 'channel'
   AND kind = 'custom'
   AND trigger = '!stalk'
   AND channel_twitch_id = '1099840105';
